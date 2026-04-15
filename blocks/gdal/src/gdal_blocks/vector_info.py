"""Block: gdal.vector_info — wraps gdal.VectorInfo(format='json')."""
from __future__ import annotations

import json

from osgeo import gdal
from spade import JsonFile, VectorFile, run

from gdal_blocks._common import output_path


def handler(source: VectorFile) -> JsonFile:
    metadata = gdal.VectorInfo(source.path, format="json")
    if metadata is None:
        raise RuntimeError(f"gdal.VectorInfo failed for {source.path}")

    out = output_path("vector_info", "json")
    with open(out, "w") as f:
        json.dump(metadata, f, indent=2, default=str)
    return JsonFile(path=out)


if __name__ == "__main__":
    run(handler)
