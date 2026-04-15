"""Block: gdal.retile — split a raster into a regular grid of tiles."""
from __future__ import annotations

from pathlib import Path

from osgeo import gdal
from spade import RasterFile, RasterFileCollection, run


def handler(
    source: RasterFile,
    tile_width: int = 256,
    tile_height: int = 256,
) -> RasterFileCollection:
    out_dir = Path("tiles")
    out_dir.mkdir(parents=True, exist_ok=True)

    ds = gdal.Open(source.path)
    if ds is None:
        raise RuntimeError(f"could not open {source.path}")

    xsize, ysize = ds.RasterXSize, ds.RasterYSize
    paths: list[str] = []
    index = 0
    for y in range(0, ysize, tile_height):
        for x in range(0, xsize, tile_width):
            w = min(tile_width, xsize - x)
            h = min(tile_height, ysize - y)
            out = out_dir / f"tile_{index:05d}.tif"
            sub = gdal.Translate(
                str(out),
                ds,
                srcWin=[x, y, w, h],
                format="GTiff",
            )
            if sub is None:
                raise RuntimeError(f"gdal.Translate failed for window {x},{y}")
            sub = None
            paths.append(str(out))
            index += 1

    ds = None
    return RasterFileCollection(paths=paths)


if __name__ == "__main__":
    run(handler)
