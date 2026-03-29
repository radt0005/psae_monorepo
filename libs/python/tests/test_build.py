from spade.build import build
from spade.types import (
    Directory,
    File,
    FileCollection,
    JsonFile,
    RasterFile,
    RasterFileCollection,
    VectorFile,
)


class TestBuild:
    def test_simple_function(self):
        def handler(source: File):
            pass

        manifest = build(handler)
        assert "inputs" in manifest
        assert "source" in manifest["inputs"]
        assert manifest["inputs"]["source"]["type"] == "file"

    def test_typed_inputs(self):
        def handler(raster: RasterFile, vector: VectorFile):
            pass

        manifest = build(handler)
        assert manifest["inputs"]["raster"] == {"type": "file", "format": "GeoTIFF"}
        assert manifest["inputs"]["vector"] == {"type": "file", "format": "GeoJSON"}

    def test_scalar_inputs(self):
        def handler(name: str, count: int, enabled: bool):
            pass

        manifest = build(handler)
        assert manifest["inputs"]["name"]["type"] == "string"
        assert manifest["inputs"]["count"]["type"] == "number"
        assert manifest["inputs"]["enabled"]["type"] == "boolean"

    def test_with_return_type(self):
        def handler(source: File) -> RasterFile:
            pass

        manifest = build(handler)
        assert "outputs" in manifest
        assert "raster" in manifest["outputs"]
        assert manifest["outputs"]["raster"]["type"] == "file"
        assert manifest["outputs"]["raster"]["format"] == "GeoTIFF"

    def test_with_docstring(self):
        def handler(source: File):
            """Processes input data."""
            pass

        manifest = build(handler)
        assert manifest["description"] == "Processes input data."

    def test_no_type_hints(self):
        def handler(source):
            pass

        manifest = build(handler)
        assert manifest["inputs"] == {}
        assert manifest["outputs"] == {}

    def test_collection_input(self):
        def handler(tiles: RasterFileCollection):
            pass

        manifest = build(handler)
        assert manifest["inputs"]["tiles"]["type"] == "collection"
        assert manifest["inputs"]["tiles"]["item_type"] == "file"
        assert manifest["inputs"]["tiles"]["format"] == "GeoTIFF"

    def test_no_return_type(self):
        def handler(source: File):
            pass

        manifest = build(handler)
        assert manifest["outputs"] == {}

    def test_none_return_type(self):
        def handler(source: File) -> None:
            pass

        manifest = build(handler)
        assert manifest["outputs"] == {}

    def test_float_input(self):
        def handler(value: float):
            pass

        manifest = build(handler)
        assert manifest["inputs"]["value"]["type"] == "number"

    def test_mixed_inputs(self):
        def handler(raster: RasterFile, buffer: int, normalize: bool) -> RasterFile:
            """Normalizes raster data."""
            pass

        manifest = build(handler)
        assert len(manifest["inputs"]) == 3
        assert manifest["inputs"]["raster"]["type"] == "file"
        assert manifest["inputs"]["buffer"]["type"] == "number"
        assert manifest["inputs"]["normalize"]["type"] == "boolean"
        assert manifest["outputs"]["raster"]["type"] == "file"
        assert manifest["description"] == "Normalizes raster data."
