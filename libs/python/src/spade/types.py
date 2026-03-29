from pydantic import BaseModel


class File(BaseModel):
    """Base class for single-file inputs/outputs."""
    path: str


class Directory(BaseModel):
    """Base class for directory-based inputs/outputs."""
    path: str


class RasterFile(File):
    """Raster data file (e.g., GeoTIFF)."""
    pass


class VectorFile(File):
    """Vector data file (e.g., GeoJSON, Shapefile)."""
    pass


class TabularFile(File):
    """Tabular data file (e.g., CSV, Parquet)."""
    pass


class JsonFile(File):
    """JSON data file."""
    pass


class FileCollection(BaseModel):
    """Base class for a collection of files."""
    paths: list[str]


class RasterFileCollection(FileCollection):
    """Collection of raster data files."""
    pass


class VectorFileCollection(FileCollection):
    """Collection of vector data files."""
    pass


class TabularFileCollection(FileCollection):
    """Collection of tabular data files."""
    pass
