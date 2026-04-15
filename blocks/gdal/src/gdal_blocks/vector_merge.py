"""Block: gdal.vector_merge — append multiple vector files into one.

Uses OGR's layer copy primitive directly rather than ``gdal.VectorTranslate``
so that format drivers without multi-layer support (e.g. GeoJSON) still
produce a single flat layer containing every source's features.
"""
from __future__ import annotations

from osgeo import ogr
from spade import VectorFile, VectorFileCollection, run

from gdal_blocks._common import output_path


_EXTENSIONS = {
    "GeoJSON": "geojson",
    "ESRI Shapefile": "shp",
    "GPKG": "gpkg",
    "FlatGeobuf": "fgb",
}


def handler(
    sources: VectorFileCollection,
    output_format: str = "GeoJSON",
) -> VectorFile:
    if not sources.paths:
        raise ValueError("gdal.vector_merge requires at least one input vector")

    ext = _EXTENSIONS.get(output_format, "geojson")
    out = output_path("merged", ext)

    driver = ogr.GetDriverByName(output_format)
    if driver is None:
        raise RuntimeError(f"no OGR driver for format {output_format!r}")
    dst_ds = driver.CreateDataSource(out)

    dst_layer = None
    for path in sources.paths:
        src_ds = ogr.Open(path)
        if src_ds is None:
            raise RuntimeError(f"could not open {path}")
        src_layer = src_ds.GetLayer(0)

        if dst_layer is None:
            dst_layer = dst_ds.CreateLayer(
                "merged",
                srs=src_layer.GetSpatialRef(),
                geom_type=src_layer.GetGeomType(),
            )
            src_defn = src_layer.GetLayerDefn()
            for i in range(src_defn.GetFieldCount()):
                dst_layer.CreateField(src_defn.GetFieldDefn(i))

        dst_defn = dst_layer.GetLayerDefn()
        for feat in src_layer:
            new_feat = ogr.Feature(dst_defn)
            new_feat.SetGeometry(feat.GetGeometryRef().Clone())
            for i in range(dst_defn.GetFieldCount()):
                name = dst_defn.GetFieldDefn(i).GetNameRef()
                if feat.GetFieldIndex(name) >= 0:
                    new_feat.SetField(name, feat.GetField(name))
            dst_layer.CreateFeature(new_feat)
            new_feat = None
        src_ds = None

    dst_ds = None
    return VectorFile(path=out)


if __name__ == "__main__":
    run(handler)
