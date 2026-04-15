"""Block: gdal.map_raster_tiles — emit an expansion manifest over a raster collection.

The scheduler reads ``outputs/manifest/expansion.yaml`` and dispatches one
downstream invocation per listed item, with the tile symlinked as the
downstream block's input.
"""
from __future__ import annotations

from pathlib import Path

import yaml
from spade import File, RasterFileCollection, run


def handler(sources: RasterFileCollection) -> File:
    if not sources.paths:
        raise ValueError("gdal.map_raster_tiles requires at least one tile")

    items = []
    for i, path in enumerate(sorted(sources.paths)):
        key = Path(path).stem or f"tile_{i:05d}"
        items.append({"path": path, "key": key})

    out = Path("expansion.yaml")
    with open(out, "w") as f:
        yaml.safe_dump({"items": items}, f, sort_keys=False)
    return File(path=str(out))


if __name__ == "__main__":
    run(handler)
