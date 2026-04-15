"""Block: gdal.location_info — read raster band values at a single point."""
from __future__ import annotations

import json

from osgeo import gdal
from spade import JsonFile, RasterFile, run

from gdal_blocks._common import output_path


def handler(
    source: RasterFile,
    x: float,
    y: float,
    coord_system: str = "georef",
) -> JsonFile:
    if coord_system not in ("georef", "pixel"):
        raise ValueError("coord_system must be 'georef' or 'pixel'")

    ds = gdal.Open(source.path)
    if ds is None:
        raise RuntimeError(f"could not open {source.path}")

    if coord_system == "georef":
        gt = ds.GetGeoTransform()
        inv = gdal.InvGeoTransform(gt)
        px, py = gdal.ApplyGeoTransform(inv, x, y)
        px_i = int(px)
        py_i = int(py)
    else:
        px_i = int(x)
        py_i = int(y)

    if not (0 <= px_i < ds.RasterXSize and 0 <= py_i < ds.RasterYSize):
        raise ValueError(
            f"point ({x}, {y}) falls outside raster bounds"
        )

    values = {}
    for band_idx in range(1, ds.RasterCount + 1):
        band = ds.GetRasterBand(band_idx)
        value = band.ReadAsArray(px_i, py_i, 1, 1)[0, 0]
        values[str(band_idx)] = float(value)
    ds = None

    out = output_path("location_info", "json")
    with open(out, "w") as f:
        json.dump(
            {"x": x, "y": y, "pixel": [px_i, py_i], "values": values},
            f,
            indent=2,
        )
    return JsonFile(path=out)


if __name__ == "__main__":
    run(handler)
