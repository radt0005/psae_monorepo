"""Block: gdal.roughness."""
from __future__ import annotations

from spade import RasterFile, run

from gdal_blocks._terrain import dem_process


def handler(source: RasterFile) -> RasterFile:
    out = dem_process("Roughness", source.path, "roughness")
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
