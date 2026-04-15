"""Shared helper for the ``ogr_layer_algebra`` family of blocks."""
from __future__ import annotations

from typing import Callable

from osgeo import ogr

from gdal_blocks._common import output_path


def run_layer_op(
    op: str,
    a_path: str,
    b_path: str,
    out_name: str,
) -> str:
    """Run a named ``ogr.Layer.<op>`` between two vector files and return the output path."""
    a_ds = ogr.Open(a_path)
    if a_ds is None:
        raise RuntimeError(f"could not open {a_path}")
    b_ds = ogr.Open(b_path)
    if b_ds is None:
        raise RuntimeError(f"could not open {b_path}")

    a_layer = a_ds.GetLayer(0)
    b_layer = b_ds.GetLayer(0)

    out = output_path(out_name, "geojson")
    driver = ogr.GetDriverByName("GeoJSON")
    out_ds = driver.CreateDataSource(out)
    out_layer = out_ds.CreateLayer(
        op.lower(),
        srs=a_layer.GetSpatialRef(),
        geom_type=a_layer.GetGeomType(),
    )

    method: Callable = getattr(a_layer, op)
    method(b_layer, out_layer)

    out_ds = None
    a_ds = None
    b_ds = None
    return out
