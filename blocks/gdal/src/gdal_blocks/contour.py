"""Block: gdal.contour — wraps gdal.ContourGenerateEx."""
from __future__ import annotations

from osgeo import gdal, ogr, osr
from spade import RasterFile, VectorFile, run

from gdal_blocks._common import output_path


def handler(
    source: RasterFile,
    band: int = 1,
    interval: float = 1.0,
    base: float = 0.0,
    field_name: str = "elev",
) -> VectorFile:
    if interval <= 0:
        raise ValueError("contour interval must be > 0")

    src = gdal.Open(source.path)
    if src is None:
        raise RuntimeError(f"could not open {source.path}")
    src_band = src.GetRasterBand(band)

    out = output_path("contours", "geojson")
    driver = ogr.GetDriverByName("GeoJSON")
    ds = driver.CreateDataSource(out)

    srs = osr.SpatialReference()
    proj = src.GetProjection()
    if proj:
        srs.ImportFromWkt(proj)
    else:
        srs = None

    layer = ds.CreateLayer("contours", srs=srs, geom_type=ogr.wkbLineString)
    id_def = ogr.FieldDefn("id", ogr.OFTInteger)
    layer.CreateField(id_def)
    elev_def = ogr.FieldDefn(field_name, ogr.OFTReal)
    layer.CreateField(elev_def)
    id_idx = layer.GetLayerDefn().GetFieldIndex("id")
    elev_idx = layer.GetLayerDefn().GetFieldIndex(field_name)

    options = [
        f"LEVEL_INTERVAL={interval}",
        f"LEVEL_BASE={base}",
        f"ID_FIELD={id_idx}",
        f"ELEV_FIELD={elev_idx}",
    ]
    gdal.ContourGenerateEx(src_band, layer, options=options)

    ds = None  # close
    src = None
    return VectorFile(path=out)


if __name__ == "__main__":
    run(handler)
