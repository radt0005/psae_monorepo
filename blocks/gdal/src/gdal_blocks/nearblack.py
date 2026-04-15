"""Block: gdal.nearblack — wraps gdal.Nearblack."""
from __future__ import annotations

from osgeo import gdal
from spade import RasterFile, run

from gdal_blocks._common import output_path


def handler(
    source: RasterFile,
    near: int = 15,
    white: bool = False,
    set_alpha: bool = False,
) -> RasterFile:
    out = output_path("nearblack", "tif")
    ds = gdal.Nearblack(
        out,
        source.path,
        format="GTiff",
        nearDist=near,
        white=white,
        setAlpha=set_alpha,
    )
    if ds is None:
        raise RuntimeError(f"gdal.Nearblack failed for {source.path}")
    ds = None  # close
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
