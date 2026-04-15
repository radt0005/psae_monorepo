"""Block: gdal.calc — evaluate a NumPy expression over a raster.

Uses numpy directly instead of ``osgeo_utils.gdal_calc`` so the block is
easier to drive from Python without touching argv. The expression variable
is ``A`` (the source raster's first band).
"""
from __future__ import annotations

import numpy as np
from osgeo import gdal
from spade import RasterFile, run

from gdal_blocks._common import output_path


_SAFE_NP_NAMES = {
    name: getattr(np, name)
    for name in (
        "where", "log", "log10", "exp", "sqrt", "abs",
        "minimum", "maximum", "clip",
        "sin", "cos", "tan", "arcsin", "arccos", "arctan", "arctan2",
        "pi",
    )
}


def handler(
    source: RasterFile,
    expression: str,
    output_type: str = "Float32",
    nodata: float | None = None,
) -> RasterFile:
    if not expression:
        raise ValueError("calc: 'expression' is required")

    ds = gdal.Open(source.path)
    if ds is None:
        raise RuntimeError(f"could not open {source.path}")
    band = ds.GetRasterBand(1)
    A = band.ReadAsArray()

    env = {"A": A, **_SAFE_NP_NAMES}
    # eval on a numpy array with a tiny vocabulary; no builtins exposed.
    result = eval(expression, {"__builtins__": {}}, env)  # noqa: S307
    result = np.asarray(result)

    dtype_code = gdal.GetDataTypeByName(output_type)
    if dtype_code == 0:
        raise ValueError(f"unknown output_type {output_type!r}")

    out = output_path("calc", "tif")
    driver = gdal.GetDriverByName("GTiff")
    dst = driver.Create(out, ds.RasterXSize, ds.RasterYSize, 1, dtype_code)
    dst.SetGeoTransform(ds.GetGeoTransform())
    dst.SetProjection(ds.GetProjection())
    out_band = dst.GetRasterBand(1)
    if nodata is not None:
        out_band.SetNoDataValue(nodata)
    out_band.WriteArray(result)
    out_band.FlushCache()
    dst = None
    ds = None
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
