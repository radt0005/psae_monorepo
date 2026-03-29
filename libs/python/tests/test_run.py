import yaml
import pytest

from spade.run import run
from spade.types import Directory, File, FileCollection, JsonFile, RasterFile


class TestRun:
    def test_simple_handler(self, work_dir, create_input_file):
        create_input_file("source", "data.tif")
        called_with = {}

        def handler(source: File):
            called_with["source"] = source

        run(handler)
        assert "source" in called_with
        assert isinstance(called_with["source"], File)

    def test_with_params_and_inputs(self, work_dir, create_input_file):
        with open(work_dir / "params.yaml", "w") as f:
            yaml.dump({"buffer": 30, "method": "bilinear"}, f)
        create_input_file("raster", "data.tif")

        called_with = {}

        def handler(raster: RasterFile, buffer: int, method: str):
            called_with.update(
                {"raster": raster, "buffer": buffer, "method": method}
            )

        run(handler)
        assert isinstance(called_with["raster"], RasterFile)
        assert called_with["buffer"] == 30
        assert called_with["method"] == "bilinear"

    def test_with_typed_inputs(self, work_dir, create_input_file):
        create_input_file("image", "satellite.tif")
        called_with = {}

        def handler(image: RasterFile):
            called_with["image"] = image

        run(handler)
        assert isinstance(called_with["image"], RasterFile)

    def test_with_output(self, work_dir, create_input_file):
        create_input_file("source", "data.tif")
        result_path = work_dir / "processed.tif"
        result_path.write_bytes(b"processed data")

        def handler(source: RasterFile) -> RasterFile:
            return RasterFile(path=str(result_path))

        run(handler)
        output_files = [f for f in (work_dir / "outputs").rglob("*") if f.is_file()]
        assert len(output_files) == 1
        assert output_files[0].read_bytes() == b"processed data"

    def test_with_dict_output(self, work_dir, create_input_file):
        create_input_file("source", "data.tif")
        raster_path = work_dir / "result.tif"
        raster_path.write_bytes(b"raster")
        json_path = work_dir / "stats.json"
        json_path.write_bytes(b'{"mean": 42}')

        def handler(source: File) -> dict:
            return {
                "raster": RasterFile(path=str(raster_path)),
                "stats": File(path=str(json_path)),
            }

        run(handler)
        assert (work_dir / "outputs" / "raster" / "result.tif").exists()
        assert (work_dir / "outputs" / "stats" / "stats.json").exists()

    def test_handler_exception_propagates(self, work_dir, create_input_file):
        create_input_file("source", "data.tif")

        def handler(source: File):
            raise RuntimeError("processing failed")

        with pytest.raises(RuntimeError, match="processing failed"):
            run(handler)

    def test_no_return_value(self, work_dir, create_input_file):
        create_input_file("source", "data.tif")

        def handler(source: File):
            pass

        run(handler)
        output_files = [f for f in (work_dir / "outputs").rglob("*") if f.is_file()]
        assert len(output_files) == 0

    def test_filters_extra_params(self, work_dir, create_input_file):
        with open(work_dir / "params.yaml", "w") as f:
            yaml.dump({"expected": "value", "extra": "ignored"}, f)
        create_input_file("source", "data.tif")

        called_with = {}

        def handler(source: File, expected: str):
            called_with["expected"] = expected

        run(handler)
        assert called_with["expected"] == "value"

    def test_full_workflow(self, work_dir, create_input_file):
        """End-to-end test simulating a real block execution."""
        with open(work_dir / "params.yaml", "w") as f:
            yaml.dump({"resolution": 10, "method": "nearest"}, f)
        create_input_file("reference", "ref.tif", b"reference raster data")
        create_input_file("target", "tgt.tif", b"target raster data")

        result_path = work_dir / "reprojected.tif"
        result_path.write_bytes(b"reprojected output")

        def handler(
            reference: RasterFile,
            target: RasterFile,
            resolution: int,
            method: str,
        ) -> RasterFile:
            assert reference.path.endswith("ref.tif")
            assert target.path.endswith("tgt.tif")
            assert resolution == 10
            assert method == "nearest"
            return RasterFile(path=str(result_path))

        run(handler)

        output_files = list((work_dir / "outputs").rglob("*.tif"))
        assert len(output_files) == 1
        assert output_files[0].read_bytes() == b"reprojected output"
