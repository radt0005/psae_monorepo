"""Block: gdal.aspect."""
from __future__ import annotations

from spade import RasterFile, run

from gdal_blocks._terrain import dem_process


def handler(
    source: RasterFile,
    zero_for_flat: bool = False,
    trigonometric: bool = False,
) -> RasterFile:
    out = dem_process(
        "aspect",
        source.path,
        "aspect",
        zeroForFlat=zero_for_flat,
        trigonometric=trigonometric,
    )
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
