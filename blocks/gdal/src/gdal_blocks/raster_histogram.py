"""Block: gdal.raster_histogram — bin raster values into classes and tabulate area.

Replaces the per-county CHM height-class tabulation in the reference workflow
(03_Calculate_NAIP_Raster_Metrics.R / x03_Calculate_GEDI_Raster_Metrics.R):
bin the valid pixels of a single-band raster into right-closed classes and
report pixel count and ground area per class. Bins match R's ``cut()``
semantics: interval ``(lower, upper]``, so a value equal to the lowest edge or
above the highest edge is excluded.
"""
from __future__ import annotations

import csv

import numpy as np
from osgeo import gdal
from spade import RasterFile, TabularFile, run

from gdal_blocks._common import output_path


def _parse_floats(text: str) -> list[float]:
    return [float(p) for p in (s.strip() for s in text.split(",")) if p]


def _parse_labels(text: str) -> list[str]:
    return [s.strip() for s in text.split(",") if s.strip()]


def handler(
    source: RasterFile,
    breaks: str,
    labels: str = "",
    min_valid: float | None = None,
    max_valid: float | None = None,
    area_scale: float = 1.0,
) -> TabularFile:
    edges = _parse_floats(breaks)
    if len(edges) < 2:
        raise ValueError("raster_histogram: 'breaks' needs at least two edges")
    if any(b <= a for a, b in zip(edges, edges[1:])):
        raise ValueError("raster_histogram: 'breaks' must be strictly ascending")
    n_bins = len(edges) - 1

    label_list = _parse_labels(labels)
    if label_list and len(label_list) != n_bins:
        raise ValueError(
            f"raster_histogram: got {len(label_list)} labels for {n_bins} bins"
        )
    if not label_list:
        # Default: label each bin by its upper edge (the workflow's 5, 10, ... 35).
        label_list = [_fmt_num(edges[i + 1]) for i in range(n_bins)]

    ds = gdal.Open(source.path)
    if ds is None:
        raise RuntimeError(f"could not open {source.path}")
    band = ds.GetRasterBand(1)
    arr = band.ReadAsArray()
    if arr is None:
        raise RuntimeError(f"could not read band 1 of {source.path}")
    gt = ds.GetGeoTransform()
    nodata = band.GetNoDataValue()

    flat = np.asarray(arr, dtype="float64").ravel()

    # Build the valid-pixel mask: finite, not nodata, within [min_valid, max_valid].
    valid = np.isfinite(flat)
    if nodata is not None:
        valid &= flat != nodata
    if min_valid is not None:
        valid &= flat >= min_valid
    if max_valid is not None:
        valid &= flat <= max_valid
    values = flat[valid]

    # Right-closed bins (lower, upper], matching R's cut(): digitize with
    # right=True returns index i for edges[i-1] < x <= edges[i]. Indices 1..n_bins
    # map to our bins; 0 (<= first edge) and n_bins+1 (> last edge) fall outside.
    idx = np.digitize(values, edges, right=True)
    counts = np.bincount(idx, minlength=len(edges) + 1)[1 : n_bins + 1]

    pixel_area = abs(gt[1] * gt[5]) * float(area_scale)

    out = output_path("histogram", "csv")
    with open(out, "w", newline="") as f:
        writer = csv.writer(f)
        writer.writerow(["label", "lower", "upper", "count", "area"])
        for i in range(n_bins):
            count = int(counts[i])
            writer.writerow([
                label_list[i],
                _fmt_num(edges[i]),
                _fmt_num(edges[i + 1]),
                count,
                count * pixel_area,
            ])
    return TabularFile(path=out)


def _fmt_num(x: float) -> str:
    """Render an edge/label as an int when it is integral, else as a float."""
    return str(int(x)) if float(x).is_integer() else repr(float(x))


if __name__ == "__main__":
    run(handler)
