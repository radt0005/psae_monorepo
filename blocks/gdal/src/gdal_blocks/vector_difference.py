"""Block: gdal.vector_difference.

OGR exposes a single primitive for A \\ B under the name ``Erase``; this
block is the 'difference' naming for users who don't come from an
ArcGIS background. See ``gdal.vector_erase`` for the equivalent name.
"""
from __future__ import annotations

from spade import VectorFile, run

from gdal_blocks._vector_algebra import run_layer_op


def handler(a: VectorFile, b: VectorFile) -> VectorFile:
    out = run_layer_op("Erase", a.path, b.path, "difference")
    return VectorFile(path=out)


if __name__ == "__main__":
    run(handler)
