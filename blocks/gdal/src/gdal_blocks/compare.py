"""Block: gdal.compare — summarise structural and pixel-level differences."""
from __future__ import annotations

import json

import numpy as np
from osgeo import gdal
from spade import JsonFile, RasterFile, run

from gdal_blocks._common import output_path


def _open(path: str) -> gdal.Dataset:
    ds = gdal.Open(path)
    if ds is None:
        raise RuntimeError(f"could not open {path}")
    return ds


def handler(golden: RasterFile, new: RasterFile) -> JsonFile:
    a = _open(golden.path)
    b = _open(new.path)

    differences: list[str] = []
    if (a.RasterXSize, a.RasterYSize) != (b.RasterXSize, b.RasterYSize):
        differences.append(
            f"size differs: {(a.RasterXSize, a.RasterYSize)} vs {(b.RasterXSize, b.RasterYSize)}"
        )
    if a.RasterCount != b.RasterCount:
        differences.append(
            f"band count differs: {a.RasterCount} vs {b.RasterCount}"
        )
    if a.GetProjection() != b.GetProjection():
        differences.append("projection differs")
    if a.GetGeoTransform() != b.GetGeoTransform():
        differences.append("geotransform differs")

    pixel_report = None
    if not differences:
        total = 0
        different = 0
        max_abs_diff = 0.0
        for i in range(1, a.RasterCount + 1):
            a_arr = a.GetRasterBand(i).ReadAsArray().astype(np.float64)
            b_arr = b.GetRasterBand(i).ReadAsArray().astype(np.float64)
            diff = np.abs(a_arr - b_arr)
            total += diff.size
            different += int((diff > 0).sum())
            max_abs_diff = max(max_abs_diff, float(diff.max()))
        pixel_report = {
            "total_pixels": total,
            "different_pixels": different,
            "max_abs_difference": max_abs_diff,
        }

    a = None
    b = None

    out = output_path("compare", "json")
    with open(out, "w") as f:
        json.dump(
            {
                "match": not differences and (pixel_report and pixel_report["different_pixels"] == 0),
                "structural_differences": differences,
                "pixel_report": pixel_report,
            },
            f,
            indent=2,
        )
    return JsonFile(path=out)


if __name__ == "__main__":
    run(handler)
