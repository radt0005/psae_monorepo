"""Block: gdal.slope."""
from __future__ import annotations

from spade import RasterFile, run

from gdal_blocks._terrain import dem_process


def handler(
    source: RasterFile,
    scale: float = 1.0,
    slope_format: str = "degree",
) -> RasterFile:
    if slope_format not in ("degree", "percent"):
        raise ValueError("slope_format must be 'degree' or 'percent'")
    out = dem_process(
        "slope",
        source.path,
        "slope",
        scale=scale,
        slopeFormat=slope_format,
    )
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
