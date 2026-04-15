"""Block: gdal.clip_raster_by_extent — wrap gdalwarp with outputBounds."""
from __future__ import annotations

from osgeo import gdal
from spade import RasterFile, run

from gdal_blocks._common import output_path


def handler(
    source: RasterFile,
    xmin: float,
    ymin: float,
    xmax: float,
    ymax: float,
) -> RasterFile:
    if xmin >= xmax or ymin >= ymax:
        raise ValueError("clip extent requires xmin < xmax and ymin < ymax")

    out = output_path("clipped", "tif")
    ds = gdal.Warp(
        out,
        source.path,
        format="GTiff",
        outputBounds=[xmin, ymin, xmax, ymax],
    )
    if ds is None:
        raise RuntimeError("gdal.Warp failed during clip_raster_by_extent")
    ds = None
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
