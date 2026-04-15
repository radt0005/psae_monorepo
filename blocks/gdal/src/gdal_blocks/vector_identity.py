"""Block: gdal.vector_identity."""
from __future__ import annotations

from spade import VectorFile, run

from gdal_blocks._vector_algebra import run_layer_op


def handler(a: VectorFile, b: VectorFile) -> VectorFile:
    out = run_layer_op("Identity", a.path, b.path, "identity")
    return VectorFile(path=out)


if __name__ == "__main__":
    run(handler)
