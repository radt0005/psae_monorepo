import os

import yaml
import pytest

from spade._output import _infer_output_name, read_block_manifest, write_outputs
from spade.types import (
    Directory,
    File,
    FileCollection,
    JsonFile,
    RasterFile,
    RasterFileCollection,
    VectorFile,
)


class TestInferOutputName:
    def test_file(self):
        assert _infer_output_name(File(path="/tmp/a")) == "file"

    def test_raster_file(self):
        assert _infer_output_name(RasterFile(path="/tmp/a")) == "raster"

    def test_vector_file(self):
        assert _infer_output_name(VectorFile(path="/tmp/a")) == "vector"

    def test_json_file(self):
        assert _infer_output_name(JsonFile(path="/tmp/a")) == "json"

    def test_file_collection(self):
        assert _infer_output_name(FileCollection(paths=[])) == "files"

    def test_raster_file_collection(self):
        assert _infer_output_name(RasterFileCollection(paths=[])) == "rasters"


class TestWriteOutputs:
    def test_none_result(self, work_dir):
        write_outputs(None)
        assert list((work_dir / "outputs").iterdir()) == []

    def test_single_file_output(self, work_dir):
        src = work_dir / "tmp_result.tif"
        src.write_bytes(b"raster data")

        write_outputs(RasterFile(path=str(src)))
        output_dir = work_dir / "outputs" / "raster"
        assert output_dir.exists()
        assert (output_dir / "tmp_result.tif").exists()
        assert (output_dir / "tmp_result.tif").read_bytes() == b"raster data"

    def test_single_file_with_manifest(self, work_dir):
        src = work_dir / "result.tif"
        src.write_bytes(b"data")
        manifest = {"custom_output": {"type": "file", "format": "GeoTIFF"}}
        write_outputs(RasterFile(path=str(src)), manifest_outputs=manifest)
        assert (work_dir / "outputs" / "custom_output" / "result.tif").exists()

    def test_dict_output(self, work_dir):
        src_raster = work_dir / "result.tif"
        src_raster.write_bytes(b"raster")
        src_json = work_dir / "summary.json"
        src_json.write_bytes(b'{"key": "value"}')

        write_outputs({
            "raster": RasterFile(path=str(src_raster)),
            "summary": JsonFile(path=str(src_json)),
        })

        assert (work_dir / "outputs" / "raster" / "result.tif").exists()
        assert (work_dir / "outputs" / "summary" / "summary.json").exists()

    def test_collection_output(self, work_dir):
        paths = []
        for i in range(3):
            p = work_dir / f"tile_{i}.tif"
            p.write_bytes(f"tile {i}".encode())
            paths.append(str(p))

        write_outputs(RasterFileCollection(paths=paths))
        output_dir = work_dir / "outputs" / "rasters"
        assert output_dir.exists()
        assert len(list(output_dir.iterdir())) == 3

    def test_directory_output(self, work_dir):
        src_dir = work_dir / "result_dir"
        src_dir.mkdir()
        (src_dir / "file1.txt").write_bytes(b"a")
        (src_dir / "file2.txt").write_bytes(b"b")

        write_outputs(Directory(path=str(src_dir)))
        output_dir = work_dir / "outputs" / "directory"
        assert output_dir.exists()
        assert (output_dir / "file1.txt").exists()
        assert (output_dir / "file2.txt").exists()

    def test_preserves_filename(self, work_dir):
        src = work_dir / "my_custom_name.geojson"
        src.write_bytes(b"geojson data")
        write_outputs(VectorFile(path=str(src)))
        assert (work_dir / "outputs" / "vector" / "my_custom_name.geojson").exists()


class TestReadBlockManifest:
    def test_no_manifest(self, work_dir):
        assert read_block_manifest() is None

    def test_block_yaml_in_cwd(self, work_dir):
        manifest = {
            "id": "test.block",
            "outputs": {"raster": {"type": "file", "format": "GeoTIFF"}},
        }
        with open(work_dir / "block.yaml", "w") as f:
            yaml.dump(manifest, f)
        result = read_block_manifest()
        assert result == {"raster": {"type": "file", "format": "GeoTIFF"}}

    def test_env_var_manifest(self, work_dir, tmp_path):
        manifest_path = tmp_path / "external" / "block.yaml"
        manifest_path.parent.mkdir(exist_ok=True)
        manifest = {
            "outputs": {"output": {"type": "file"}},
        }
        with open(manifest_path, "w") as f:
            yaml.dump(manifest, f)
        os.environ["SPADE_BLOCK_MANIFEST"] = str(manifest_path)
        try:
            result = read_block_manifest()
            assert result == {"output": {"type": "file"}}
        finally:
            del os.environ["SPADE_BLOCK_MANIFEST"]
