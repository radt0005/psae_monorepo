"""Block: gdal.reduce_mosaic — fan-in mosaic via gdal.Warp."""
from __future__ import annotations

from osgeo import gdal
from spade import RasterFile, RasterFileCollection, run

from gdal_blocks._common import output_path
from gdal_blocks._resampling import warp_method


def handler(
    tiles: RasterFileCollection,
    resampling: str = "nearest",
) -> RasterFile:
    if not tiles.paths:
        raise ValueError("gdal.reduce_mosaic requires at least one tile")

    out = output_path("mosaic", "tif")
    ds = gdal.Warp(
        out,
        list(tiles.paths),
        format="GTiff",
        resampleAlg=warp_method(resampling),
    )
    if ds is None:
        raise RuntimeError("gdal.Warp failed during reduce_mosaic")
    ds = None
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
