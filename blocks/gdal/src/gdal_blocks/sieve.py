"""Block: gdal.sieve — wraps gdal.SieveFilter."""
from __future__ import annotations

import shutil

from osgeo import gdal
from spade import RasterFile, run

from gdal_blocks._common import output_path


def handler(
    source: RasterFile,
    threshold: int = 4,
    connectedness: int = 4,
) -> RasterFile:
    if connectedness not in (4, 8):
        raise ValueError("connectedness must be 4 or 8")
    if threshold < 1:
        raise ValueError("threshold must be >= 1")

    out = output_path("sieved", "tif")
    shutil.copy2(source.path, out)

    ds = gdal.Open(out, gdal.GA_Update)
    if ds is None:
        raise RuntimeError(f"could not open {out} for update")
    band = ds.GetRasterBand(1)
    gdal.SieveFilter(band, None, band, threshold, connectedness)
    ds.FlushCache()
    ds = None
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
