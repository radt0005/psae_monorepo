"""Block: gdal.tri — Terrain Ruggedness Index."""
from __future__ import annotations

from spade import RasterFile, run

from gdal_blocks._terrain import dem_process


def handler(source: RasterFile, algorithm: str = "Wilson") -> RasterFile:
    if algorithm not in ("Wilson", "Riley"):
        raise ValueError("algorithm must be 'Wilson' or 'Riley'")
    out = dem_process("TRI", source.path, "tri", alg=algorithm)
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
