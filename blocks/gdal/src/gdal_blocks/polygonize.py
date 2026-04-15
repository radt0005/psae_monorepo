"""Block: gdal.polygonize — wraps gdal.Polygonize."""
from __future__ import annotations

from osgeo import gdal, ogr, osr
from spade import RasterFile, VectorFile, run

from gdal_blocks._common import output_path


def handler(
    source: RasterFile,
    band: int = 1,
    field_name: str = "DN",
    connectedness: int = 4,
) -> VectorFile:
    src = gdal.Open(source.path)
    if src is None:
        raise RuntimeError(f"could not open {source.path}")
    if connectedness not in (4, 8):
        raise ValueError("connectedness must be 4 or 8")

    src_band = src.GetRasterBand(band)
    mask_band = src_band.GetMaskBand()

    out = output_path("polygons", "geojson")
    driver = ogr.GetDriverByName("GeoJSON")
    ds = driver.CreateDataSource(out)

    srs = osr.SpatialReference()
    proj = src.GetProjection()
    if proj:
        srs.ImportFromWkt(proj)
    else:
        srs = None

    layer = ds.CreateLayer("polygons", srs=srs, geom_type=ogr.wkbPolygon)
    layer.CreateField(ogr.FieldDefn(field_name, ogr.OFTInteger))
    field_idx = layer.GetLayerDefn().GetFieldIndex(field_name)

    options = ["8CONNECTED=8"] if connectedness == 8 else []
    gdal.Polygonize(src_band, mask_band, layer, field_idx, options)

    ds = None  # close
    src = None
    return VectorFile(path=out)


if __name__ == "__main__":
    run(handler)
