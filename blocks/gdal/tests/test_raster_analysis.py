"""Tests for the raster analysis blocks (Phase 2.3)."""
from __future__ import annotations

from pathlib import Path

import numpy as np
import pytest
from osgeo import gdal
from spade import RasterFile, TabularFile

from gdal_blocks import calc, fill_nodata, grid, proximity, sieve, viewshed


# --------------------------------------------------------------------------- calc

def test_calc_doubles_values(raster_path: Path):
    result = calc.handler(
        RasterFile(path=str(raster_path)), expression="A * 2"
    )
    ds = gdal.Open(result.path)
    arr = ds.GetRasterBand(1).ReadAsArray()
    # Original ramp max is 15 + 15 = 30, doubled = 60.
    assert arr.max() == pytest.approx(60.0)


def test_calc_threshold(raster_path: Path):
    result = calc.handler(
        RasterFile(path=str(raster_path)),
        expression="(A > 20) * 1",
        output_type="Byte",
    )
    ds = gdal.Open(result.path)
    arr = ds.GetRasterBand(1).ReadAsArray()
    assert set(np.unique(arr)) <= {0, 1}


def test_calc_empty_expression_rejected(raster_path: Path):
    with pytest.raises(ValueError):
        calc.handler(RasterFile(path=str(raster_path)), expression="")


# --------------------------------------------------------------------------- sieve

def test_sieve_removes_isolated_pixels(classified_raster: Path):
    # The 99-valued single pixel should be sieved away with threshold=2.
    result = sieve.handler(
        RasterFile(path=str(classified_raster)), threshold=2
    )
    ds = gdal.Open(result.path)
    arr = ds.GetRasterBand(1).ReadAsArray()
    assert (arr == 99).sum() == 0


def test_sieve_invalid_threshold(classified_raster: Path):
    with pytest.raises(ValueError):
        sieve.handler(RasterFile(path=str(classified_raster)), threshold=0)


# --------------------------------------------------------------------------- fill_nodata

def test_fill_nodata(raster_with_nodata: Path):
    ds_before = gdal.Open(str(raster_with_nodata))
    before = ds_before.GetRasterBand(1).ReadAsArray()
    ds_before = None
    assert (before == -9999.0).sum() == 1

    result = fill_nodata.handler(
        RasterFile(path=str(raster_with_nodata)), max_distance=5.0
    )
    ds_after = gdal.Open(result.path)
    after = ds_after.GetRasterBand(1).ReadAsArray()
    ds_after = None
    # The previously-nodata pixel should now have a real value.
    assert after[8, 8] != -9999.0


def test_fill_nodata_invalid_distance(raster_with_nodata: Path):
    with pytest.raises(ValueError):
        fill_nodata.handler(
            RasterFile(path=str(raster_with_nodata)), max_distance=0
        )


# --------------------------------------------------------------------------- proximity

def test_proximity_basic(classified_raster: Path):
    result = proximity.handler(
        RasterFile(path=str(classified_raster)),
        target_values="99",
    )
    ds = gdal.Open(result.path)
    arr = ds.GetRasterBand(1).ReadAsArray()
    # The target pixel itself has distance 0; elsewhere > 0.
    assert arr.min() == 0.0
    assert arr.max() > 0.0


def test_proximity_invalid_units(classified_raster: Path):
    with pytest.raises(ValueError):
        proximity.handler(
            RasterFile(path=str(classified_raster)), distance_units="bogus"
        )


# --------------------------------------------------------------------------- grid

def test_grid_basic(points_csv: Path):
    result = grid.handler(
        TabularFile(path=str(points_csv)),
        width=16,
        height=16,
    )
    assert Path(result.path).exists()
    ds = gdal.Open(result.path)
    assert (ds.RasterXSize, ds.RasterYSize) == (16, 16)


# --------------------------------------------------------------------------- viewshed

def test_viewshed_basic(dem_path: Path):
    # Observer at the centre of the DEM.
    ds = gdal.Open(str(dem_path))
    gt = ds.GetGeoTransform()
    cx = gt[0] + gt[1] * ds.RasterXSize / 2
    cy = gt[3] + gt[5] * ds.RasterYSize / 2

    result = viewshed.handler(
        RasterFile(path=str(dem_path)),
        observer_x=cx,
        observer_y=cy,
        observer_height=10.0,
    )
    out = gdal.Open(result.path)
    arr = out.GetRasterBand(1).ReadAsArray()
    # Observer pixel should be visible.
    assert (arr == 255).any()
