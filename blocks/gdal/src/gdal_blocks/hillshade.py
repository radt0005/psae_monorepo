"""Block: gdal.hillshade."""
from __future__ import annotations

from spade import RasterFile, run

from gdal_blocks._terrain import dem_process


def handler(
    source: RasterFile,
    azimuth: float = 315.0,
    altitude: float = 45.0,
    z_factor: float = 1.0,
    scale: float = 1.0,
    multidirectional: bool = False,
) -> RasterFile:
    out = dem_process(
        "hillshade",
        source.path,
        "hillshade",
        azimuth=azimuth,
        altitude=altitude,
        zFactor=z_factor,
        scale=scale,
        multiDirectional=multidirectional,
    )
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
