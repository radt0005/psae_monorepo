"""Block: gdal.vector_tile_index — produce an index of vector footprints."""
from __future__ import annotations

from osgeo import ogr
from spade import VectorFile, VectorFileCollection, run

from gdal_blocks._common import output_path


def handler(
    sources: VectorFileCollection,
    location_field: str = "location",
) -> VectorFile:
    if not sources.paths:
        raise ValueError("gdal.vector_tile_index requires at least one input vector")

    out = output_path("vector_tile_index", "geojson")
    driver = ogr.GetDriverByName("GeoJSON")
    ds = driver.CreateDataSource(out)
    layer = ds.CreateLayer("index", geom_type=ogr.wkbPolygon)
    layer.CreateField(ogr.FieldDefn(location_field, ogr.OFTString))

    for path in sources.paths:
        src_ds = ogr.Open(path)
        if src_ds is None:
            raise RuntimeError(f"could not open {path}")
        src_layer = src_ds.GetLayer(0)
        extent = src_layer.GetExtent()  # (xmin, xmax, ymin, ymax)
        xmin, xmax, ymin, ymax = extent

        ring = ogr.Geometry(ogr.wkbLinearRing)
        ring.AddPoint(xmin, ymin)
        ring.AddPoint(xmax, ymin)
        ring.AddPoint(xmax, ymax)
        ring.AddPoint(xmin, ymax)
        ring.AddPoint(xmin, ymin)
        poly = ogr.Geometry(ogr.wkbPolygon)
        poly.AddGeometry(ring)

        feat = ogr.Feature(layer.GetLayerDefn())
        feat.SetField(location_field, path)
        feat.SetGeometry(poly)
        layer.CreateFeature(feat)
        feat = None
        src_ds = None

    ds = None
    return VectorFile(path=out)


if __name__ == "__main__":
    run(handler)
