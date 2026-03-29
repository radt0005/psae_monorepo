from spade.types import (
    Directory,
    File,
    FileCollection,
    JsonFile,
    RasterFile,
    RasterFileCollection,
    TabularFile,
    TabularFileCollection,
    VectorFile,
    VectorFileCollection,
)


class TestFile:
    def test_create_with_path(self):
        f = File(path="/tmp/data.tif")
        assert f.path == "/tmp/data.tif"

    def test_serialization(self):
        f = File(path="/tmp/data.tif")
        d = f.model_dump()
        assert d == {"path": "/tmp/data.tif"}

    def test_deserialization(self):
        f = File.model_validate({"path": "/tmp/data.tif"})
        assert f.path == "/tmp/data.tif"


class TestDirectory:
    def test_create_with_path(self):
        d = Directory(path="/tmp/source")
        assert d.path == "/tmp/source"

    def test_serialization(self):
        d = Directory(path="/tmp/source")
        assert d.model_dump() == {"path": "/tmp/source"}


class TestFileSubtypes:
    def test_raster_file_inherits_from_file(self):
        r = RasterFile(path="/tmp/raster.tif")
        assert isinstance(r, File)
        assert r.path == "/tmp/raster.tif"

    def test_vector_file_inherits_from_file(self):
        v = VectorFile(path="/tmp/vector.geojson")
        assert isinstance(v, File)
        assert v.path == "/tmp/vector.geojson"

    def test_tabular_file_inherits_from_file(self):
        t = TabularFile(path="/tmp/data.csv")
        assert isinstance(t, File)
        assert t.path == "/tmp/data.csv"

    def test_json_file_inherits_from_file(self):
        j = JsonFile(path="/tmp/data.json")
        assert isinstance(j, File)
        assert j.path == "/tmp/data.json"


class TestCollectionTypes:
    def test_file_collection(self):
        fc = FileCollection(paths=["/tmp/a.tif", "/tmp/b.tif"])
        assert fc.paths == ["/tmp/a.tif", "/tmp/b.tif"]

    def test_raster_file_collection_inherits(self):
        rc = RasterFileCollection(paths=["/tmp/a.tif", "/tmp/b.tif"])
        assert isinstance(rc, FileCollection)
        assert rc.paths == ["/tmp/a.tif", "/tmp/b.tif"]

    def test_vector_file_collection_inherits(self):
        vc = VectorFileCollection(paths=["/tmp/a.geojson"])
        assert isinstance(vc, FileCollection)

    def test_tabular_file_collection_inherits(self):
        tc = TabularFileCollection(paths=["/tmp/a.csv", "/tmp/b.csv"])
        assert isinstance(tc, FileCollection)

    def test_empty_collection(self):
        fc = FileCollection(paths=[])
        assert fc.paths == []

    def test_collection_serialization(self):
        fc = FileCollection(paths=["/tmp/a.tif", "/tmp/b.tif"])
        d = fc.model_dump()
        assert d == {"paths": ["/tmp/a.tif", "/tmp/b.tif"]}

    def test_collection_deserialization(self):
        fc = FileCollection.model_validate({"paths": ["/tmp/a.tif"]})
        assert fc.paths == ["/tmp/a.tif"]
