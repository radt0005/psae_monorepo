"""Block: gdal.merge — mosaic a collection of rasters via gdal.Warp."""
from __future__ import annotations

from osgeo import gdal
from spade import RasterFile, RasterFileCollection, run

from gdal_blocks._common import output_path
from gdal_blocks._resampling import warp_method


def handler(
    sources: RasterFileCollection,
    resampling: str = "nearest",
    output_format: str = "GTiff",
) -> RasterFile:
    if not sources.paths:
        raise ValueError("gdal.merge requires at least one input raster")

    out = output_path("mosaic", "tif")
    ds = gdal.Warp(
        out,
        list(sources.paths),
        format=output_format,
        resampleAlg=warp_method(resampling),
    )
    if ds is None:
        raise RuntimeError("gdal.Warp failed during merge")
    ds = None  # close
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
