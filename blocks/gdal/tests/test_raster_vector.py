"""Tests for the raster↔vector conversion blocks (Phase 2.2)."""
from __future__ import annotations

from pathlib import Path

import pytest
from osgeo import gdal, ogr
from spade import RasterFile, VectorFile

from gdal_blocks import contour, polygonize, rasterize


# --------------------------------------------------------------------------- rasterize

def test_rasterize_basic(vector_path: Path, raster_path: Path):
    result = rasterize.handler(
        VectorFile(path=str(vector_path)),
        RasterFile(path=str(raster_path)),
        burn_value=5.0,
    )
    assert isinstance(result, RasterFile)
    ds = gdal.Open(result.path)
    arr = ds.GetRasterBand(1).ReadAsArray()
    # Some pixels should carry the burn value.
    assert (arr == 5.0).any()
    # And the grid should match the reference raster.
    assert (ds.RasterXSize, ds.RasterYSize) == (16, 16)


def test_rasterize_with_attribute(vector_path: Path, raster_path: Path):
    result = rasterize.handler(
        VectorFile(path=str(vector_path)),
        RasterFile(path=str(raster_path)),
        attribute="value",
    )
    ds = gdal.Open(result.path)
    arr = ds.GetRasterBand(1).ReadAsArray()
    # Feature values are 10 and 20; we should see at least one of them.
    assert (arr == 10.0).any() or (arr == 20.0).any()


# --------------------------------------------------------------------------- polygonize

def test_polygonize_basic(classified_raster: Path):
    result = polygonize.handler(RasterFile(path=str(classified_raster)))
    assert isinstance(result, VectorFile)
    ds = ogr.Open(result.path)
    layer = ds.GetLayer(0)
    # At least one polygon per distinct class (1, 2, 3, 4, 99).
    assert layer.GetFeatureCount() >= 5


def test_polygonize_invalid_connectedness(classified_raster: Path):
    with pytest.raises(ValueError):
        polygonize.handler(
            RasterFile(path=str(classified_raster)), connectedness=6
        )


# --------------------------------------------------------------------------- contour

def test_contour_basic(dem_path: Path):
    result = contour.handler(
        RasterFile(path=str(dem_path)), interval=2.0
    )
    assert isinstance(result, VectorFile)
    ds = ogr.Open(result.path)
    layer = ds.GetLayer(0)
    assert layer.GetFeatureCount() > 0
    feat = layer.GetNextFeature()
    # Default field name.
    assert feat.GetField("elev") is not None


def test_contour_zero_interval_rejected(dem_path: Path):
    with pytest.raises(ValueError):
        contour.handler(RasterFile(path=str(dem_path)), interval=0.0)
