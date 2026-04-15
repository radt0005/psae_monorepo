"""Block: gdal.info — wraps gdal.Info(format='json')."""
from __future__ import annotations

import json

from osgeo import gdal
from spade import JsonFile, RasterFile, run

from gdal_blocks._common import output_path


def handler(source: RasterFile, compute_stats: bool = False) -> JsonFile:
    metadata = gdal.Info(
        source.path,
        format="json",
        stats=compute_stats,
    )
    if metadata is None:
        raise RuntimeError(f"gdal.Info failed for {source.path}")

    out = output_path("info", "json")
    with open(out, "w") as f:
        json.dump(metadata, f, indent=2, default=str)
    return JsonFile(path=out)


if __name__ == "__main__":
    run(handler)
