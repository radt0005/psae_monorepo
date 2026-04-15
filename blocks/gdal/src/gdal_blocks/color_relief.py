"""Block: gdal.color_relief."""
from __future__ import annotations

from spade import File, RasterFile, run

from gdal_blocks._terrain import dem_process


def handler(source: RasterFile, color_ramp: File) -> RasterFile:
    out = dem_process(
        "color-relief",
        source.path,
        "color_relief",
        colorFilename=color_ramp.path,
    )
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
