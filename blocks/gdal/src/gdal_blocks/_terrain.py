"""Shared helper for the gdaldem-family blocks."""
from __future__ import annotations

from osgeo import gdal

from gdal_blocks._common import output_path


def dem_process(mode: str, src_path: str, out_name: str, **kwargs) -> str:
    """Run gdal.DEMProcessing(mode, ...) and return the output path."""
    out = output_path(out_name, "tif")
    ds = gdal.DEMProcessing(out, src_path, mode, format="GTiff", **kwargs)
    if ds is None:
        raise RuntimeError(f"gdal.DEMProcessing({mode!r}) failed")
    ds = None  # close
    return out
