"""Block: gdal.tpi — Topographic Position Index."""
from __future__ import annotations

from spade import RasterFile, run

from gdal_blocks._terrain import dem_process


def handler(source: RasterFile) -> RasterFile:
    out = dem_process("TPI", source.path, "tpi")
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
