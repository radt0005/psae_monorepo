"""Block: gdal.fill_nodata — wraps gdal.FillNodata."""
from __future__ import annotations

import shutil

from osgeo import gdal
from spade import RasterFile, run

from gdal_blocks._common import output_path


def handler(
    source: RasterFile,
    max_distance: float = 100.0,
    smoothing_iterations: int = 0,
) -> RasterFile:
    if max_distance <= 0:
        raise ValueError("max_distance must be > 0")

    out = output_path("filled", "tif")
    shutil.copy2(source.path, out)

    ds = gdal.Open(out, gdal.GA_Update)
    if ds is None:
        raise RuntimeError(f"could not open {out} for update")
    band = ds.GetRasterBand(1)
    gdal.FillNodata(
        targetBand=band,
        maskBand=None,
        maxSearchDist=max_distance,
        smoothingIterations=smoothing_iterations,
    )
    ds.FlushCache()
    ds = None
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
