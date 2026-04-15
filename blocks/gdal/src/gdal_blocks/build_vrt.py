"""Block: gdal.build_vrt — wraps gdal.BuildVRT."""
from __future__ import annotations

from osgeo import gdal
from spade import RasterFile, RasterFileCollection, run

from gdal_blocks._common import output_path


def handler(
    sources: RasterFileCollection,
    resolution: str = "highest",
    separate: bool = False,
) -> RasterFile:
    if not sources.paths:
        raise ValueError("gdal.build_vrt requires at least one input raster")

    out = output_path("mosaic", "vrt")
    ds = gdal.BuildVRT(
        out,
        list(sources.paths),
        resolution=resolution,
        separate=separate,
    )
    if ds is None:
        raise RuntimeError("gdal.BuildVRT failed")
    ds = None  # close
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
