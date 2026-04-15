"""Block: gdal.reduce_vrt — fan-in VRT assembly."""
from __future__ import annotations

from osgeo import gdal
from spade import RasterFile, RasterFileCollection, run

from gdal_blocks._common import output_path


def handler(
    tiles: RasterFileCollection,
    resolution: str = "highest",
) -> RasterFile:
    if not tiles.paths:
        raise ValueError("gdal.reduce_vrt requires at least one tile")

    out = output_path("mosaic", "vrt")
    ds = gdal.BuildVRT(out, list(tiles.paths), resolution=resolution)
    if ds is None:
        raise RuntimeError("gdal.BuildVRT failed during reduce_vrt")
    ds = None
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
