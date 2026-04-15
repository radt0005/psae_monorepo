"""Sanity checks for the shared helpers and fixtures."""
from __future__ import annotations

from pathlib import Path

import pytest
from osgeo import gdal, ogr

from gdal_blocks import _common, _resampling


def test_gdal_exceptions_enabled():
    assert gdal.GetUseExceptions() == 1
    assert ogr.GetUseExceptions() == 1


def test_srs_from_user_epsg():
    srs = _common.srs_from_user("EPSG:4326")
    assert srs.GetAuthorityCode(None) == "4326"


def test_srs_from_user_int():
    srs = _common.srs_from_user(32618)
    assert srs.GetAuthorityCode(None) == "32618"


def test_srs_from_user_invalid():
    with pytest.raises(ValueError):
        _common.srs_from_user("not-a-crs")


def test_output_path():
    assert _common.output_path("warped", "tif") == "warped.tif"
    assert _common.output_path("warped", ".tif") == "warped.tif"


def test_warp_method_aliases():
    assert _resampling.warp_method("nearest") == "near"
    assert _resampling.warp_method("BILINEAR") == "bilinear"


def test_warp_method_invalid():
    with pytest.raises(ValueError):
        _resampling.warp_method("bogus")


def test_gra_constant_known():
    # Just check we get an int constant, not the specific value (version-dependent).
    assert isinstance(_resampling.gra_constant("cubic"), int)


def test_raster_fixture(raster_path: Path):
    ds = gdal.Open(str(raster_path))
    assert ds is not None
    assert ds.RasterXSize == 16
    assert ds.RasterYSize == 16
    assert ds.GetProjection() != ""


def test_vector_fixture(vector_path: Path):
    ds = ogr.Open(str(vector_path))
    assert ds is not None
    layer = ds.GetLayer(0)
    assert layer.GetFeatureCount() == 2


def test_dem_fixture(dem_path: Path):
    ds = gdal.Open(str(dem_path))
    assert ds is not None
    band = ds.GetRasterBand(1)
    arr = band.ReadAsArray()
    # DEM pattern is 100 + 0.5*x + 2*y, so values increase downward.
    assert arr[0, 0] < arr[-1, -1]


def test_classified_fixture(classified_raster: Path):
    ds = gdal.Open(str(classified_raster))
    arr = ds.GetRasterBand(1).ReadAsArray()
    assert set(int(v) for v in set(arr.flatten())) == {1, 2, 3, 4, 99}


def test_raster_collection_fixture(raster_collection):
    assert len(raster_collection) == 3
    for p in raster_collection:
        assert p.exists()


def test_invocation_dir(invocation_dir: Path):
    assert (invocation_dir / "inputs").is_dir()
    assert (invocation_dir / "outputs").is_dir()
    assert Path.cwd() == invocation_dir
