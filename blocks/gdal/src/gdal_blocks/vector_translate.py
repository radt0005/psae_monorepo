"""Block: gdal.vector_translate — wraps gdal.VectorTranslate (ogr2ogr)."""
from __future__ import annotations

from osgeo import gdal
from spade import VectorFile, run

from gdal_blocks._common import output_path


_EXTENSIONS = {
    "GeoJSON": "geojson",
    "ESRI Shapefile": "shp",
    "GPKG": "gpkg",
    "FlatGeobuf": "fgb",
    "KML": "kml",
    "CSV": "csv",
    "Parquet": "parquet",
}


def handler(
    source: VectorFile,
    output_format: str = "GeoJSON",
    target_crs: str = "",
    where: str = "",
    sql: str = "",
) -> VectorFile:
    ext = _EXTENSIONS.get(output_format, "geojson")
    out = output_path("translated", ext)

    kwargs: dict = {"format": output_format}
    if target_crs:
        kwargs["dstSRS"] = target_crs
    if sql:
        kwargs["SQLStatement"] = sql
    elif where:
        kwargs["where"] = where

    ds = gdal.VectorTranslate(out, source.path, **kwargs)
    if ds is None:
        raise RuntimeError(f"gdal.VectorTranslate failed for {source.path}")
    ds = None  # close
    return VectorFile(path=out)


if __name__ == "__main__":
    run(handler)
