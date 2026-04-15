"""Tests for the clip blocks (Phase 2.4)."""
from __future__ import annotations

from pathlib import Path

import pytest
from osgeo import gdal
from spade import RasterFile, VectorFile

from gdal_blocks import clip_raster_by_extent, clip_raster_by_vector


def test_clip_by_vector_shrinks_output(raster_path: Path, vector_path: Path):
    result = clip_raster_by_vector.handler(
        RasterFile(path=str(raster_path)),
        VectorFile(path=str(vector_path)),
    )
    src_ds = gdal.Open(str(raster_path))
    out_ds = gdal.Open(result.path)
    # The cutline covers a small area; the clipped output is smaller.
    assert out_ds.RasterXSize < src_ds.RasterXSize
    src_ds = None
    out_ds = None


def test_clip_by_extent(raster_path: Path):
    src_ds = gdal.Open(str(raster_path))
    gt = src_ds.GetGeoTransform()
    # Ask for the middle quarter.
    xmin = gt[0] + gt[1] * 4
    xmax = gt[0] + gt[1] * 12
    ymax = gt[3] + gt[5] * 4
    ymin = gt[3] + gt[5] * 12
    src_ds = None

    result = clip_raster_by_extent.handler(
        RasterFile(path=str(raster_path)),
        xmin=xmin,
        ymin=ymin,
        xmax=xmax,
        ymax=ymax,
    )
    out_ds = gdal.Open(result.path)
    assert out_ds.RasterXSize == 8
    assert out_ds.RasterYSize == 8
    out_ds = None


def test_clip_by_extent_invalid_bounds(raster_path: Path):
    with pytest.raises(ValueError):
        clip_raster_by_extent.handler(
            RasterFile(path=str(raster_path)),
            xmin=10, ymin=10, xmax=10, ymax=10,
        )
