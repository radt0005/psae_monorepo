"""Block: gdal.add_overviews — wraps Dataset.BuildOverviews."""
from __future__ import annotations

import shutil

from osgeo import gdal
from spade import RasterFile, run

from gdal_blocks._common import output_path


def handler(
    source: RasterFile,
    levels: str = "2 4 8 16",
    resampling: str = "average",
) -> RasterFile:
    out = output_path("with_overviews", "tif")
    shutil.copy2(source.path, out)

    ds = gdal.Open(out, gdal.GA_Update)
    if ds is None:
        raise RuntimeError(f"could not open {out} for update")

    overview_list = [int(v) for v in levels.split() if v]
    if not overview_list:
        raise ValueError("add_overviews: 'levels' must list at least one factor")

    ds.BuildOverviews(resampling.upper(), overview_list)
    ds.FlushCache()
    ds = None
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
