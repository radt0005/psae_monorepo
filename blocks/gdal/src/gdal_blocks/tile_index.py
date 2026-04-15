"""Block: gdal.tile_index — produce a footprint tile index for a raster collection."""
from __future__ import annotations

from osgeo import gdal, ogr
from spade import RasterFileCollection, VectorFile, run

from gdal_blocks._common import output_path


def handler(
    sources: RasterFileCollection,
    location_field: str = "location",
) -> VectorFile:
    if not sources.paths:
        raise ValueError("gdal.tile_index requires at least one input raster")

    out = output_path("tile_index", "geojson")
    driver = ogr.GetDriverByName("GeoJSON")
    ds = driver.CreateDataSource(out)
    layer = ds.CreateLayer("tile_index", geom_type=ogr.wkbPolygon)
    layer.CreateField(ogr.FieldDefn(location_field, ogr.OFTString))

    for path in sources.paths:
        raster = gdal.Open(path)
        if raster is None:
            raise RuntimeError(f"could not open {path}")

        gt = raster.GetGeoTransform()
        xsize, ysize = raster.RasterXSize, raster.RasterYSize
        x0, y0 = gt[0], gt[3]
        x1 = gt[0] + gt[1] * xsize + gt[2] * ysize
        y1 = gt[3] + gt[4] * xsize + gt[5] * ysize
        xmin, xmax = sorted((x0, x1))
        ymin, ymax = sorted((y0, y1))

        ring = ogr.Geometry(ogr.wkbLinearRing)
        ring.AddPoint(xmin, ymin)
        ring.AddPoint(xmax, ymin)
        ring.AddPoint(xmax, ymax)
        ring.AddPoint(xmin, ymax)
        ring.AddPoint(xmin, ymin)
        polygon = ogr.Geometry(ogr.wkbPolygon)
        polygon.AddGeometry(ring)

        feat = ogr.Feature(layer.GetLayerDefn())
        feat.SetField(location_field, path)
        feat.SetGeometry(polygon)
        layer.CreateFeature(feat)
        feat = None

    ds = None  # close
    return VectorFile(path=out)


if __name__ == "__main__":
    run(handler)
