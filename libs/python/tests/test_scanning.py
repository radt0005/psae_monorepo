import os

import pytest
import yaml

from spade._scanning import build_function_args, load_params, scan_inputs
from spade.types import (
    Directory,
    File,
    FileCollection,
    RasterFile,
    RasterFileCollection,
)


class TestLoadParams:
    def test_basic(self, work_dir):
        with open(work_dir / "params.yaml", "w") as f:
            yaml.dump({"buffer_distance": 30, "method": "bilinear"}, f)
        params = load_params()
        assert params == {"buffer_distance": 30, "method": "bilinear"}

    def test_empty_file(self, work_dir):
        (work_dir / "params.yaml").write_text("")
        params = load_params()
        assert params == {}

    def test_missing_file(self, work_dir):
        params = load_params()
        assert params == {}


class TestScanInputs:
    def test_single_file_input(self, work_dir, create_input_file):
        create_input_file("raster", "data.tif")
        result = scan_inputs({"raster": RasterFile})
        assert isinstance(result["raster"], RasterFile)
        assert "data.tif" in result["raster"].path

    def test_untyped_single_file_defaults_to_file(self, work_dir, create_input_file):
        create_input_file("source", "data.tif")
        result = scan_inputs({})
        assert isinstance(result["source"], File)

    def test_directory_input(self, work_dir):
        input_dir = work_dir / "inputs" / "source"
        input_dir.mkdir()
        (input_dir / "file1.shp").write_bytes(b"data")
        (input_dir / "file2.dbf").write_bytes(b"data")
        result = scan_inputs({"source": Directory})
        assert isinstance(result["source"], Directory)
        assert result["source"].path.endswith("inputs/source")

    def test_collection_input(self, work_dir, create_input_collection):
        create_input_collection("tiles", ["001.tif", "002.tif", "003.tif"])
        result = scan_inputs({"tiles": RasterFileCollection})
        assert isinstance(result["tiles"], RasterFileCollection)
        assert len(result["tiles"].paths) == 3

    def test_multiple_inputs(self, work_dir, create_input_file):
        create_input_file("reference", "ref.tif")
        create_input_file("target", "tgt.tif")
        result = scan_inputs({"reference": RasterFile, "target": RasterFile})
        assert "reference" in result
        assert "target" in result

    def test_empty_input_dir_raises(self, work_dir):
        (work_dir / "inputs" / "empty").mkdir()
        with pytest.raises(ValueError, match="empty"):
            scan_inputs({"empty": RasterFile})

    def test_untyped_multiple_files_defaults_to_collection(
        self, work_dir, create_input_collection
    ):
        create_input_collection("data", ["a.tif", "b.tif"])
        result = scan_inputs({})
        assert isinstance(result["data"], FileCollection)
        assert len(result["data"].paths) == 2

    def test_no_inputs_dir(self, tmp_path):
        original = os.getcwd()
        os.chdir(tmp_path)
        try:
            result = scan_inputs({})
            assert result == {}
        finally:
            os.chdir(original)


class TestBuildFunctionArgs:
    def test_params_and_inputs_merged(self, work_dir, create_input_file):
        with open(work_dir / "params.yaml", "w") as f:
            yaml.dump({"buffer": 30}, f)
        create_input_file("raster", "data.tif")

        def handler(raster: RasterFile, buffer: int):
            pass

        args = build_function_args(handler)
        assert "buffer" in args
        assert "raster" in args
        assert args["buffer"] == 30
        assert isinstance(args["raster"], RasterFile)

    def test_inputs_take_precedence(self, work_dir, create_input_file):
        with open(work_dir / "params.yaml", "w") as f:
            yaml.dump({"raster": "should_be_overridden"}, f)
        create_input_file("raster", "data.tif")

        def handler(raster: RasterFile):
            pass

        args = build_function_args(handler)
        assert isinstance(args["raster"], RasterFile)
