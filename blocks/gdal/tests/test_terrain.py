"""Tests for the terrain/gdaldem blocks (Phase 2.5)."""
from __future__ import annotations

from pathlib import Path

import pytest
from osgeo import gdal
from spade import File, RasterFile

from gdal_blocks import (
    aspect,
    color_relief,
    hillshade,
    roughness,
    slope,
    tpi,
    tri,
)


def _open_array(path: str):
    ds = gdal.Open(path)
    arr = ds.GetRasterBand(1).ReadAsArray()
    ds = None
    return arr


def test_hillshade(dem_path: Path):
    result = hillshade.handler(RasterFile(path=str(dem_path)))
    assert Path(result.path).exists()
    arr = _open_array(result.path)
    # Hillshade is a Byte 0-255 output.
    assert arr.min() >= 0 and arr.max() <= 255


def test_slope_degrees(dem_path: Path):
    result = slope.handler(RasterFile(path=str(dem_path)))
    arr = _open_array(result.path)
    # Interior pixels are valid slopes 0-90; edges are nodata (-9999).
    valid = arr[arr != -9999.0]
    assert valid.min() >= 0
    assert valid.max() <= 90 + 1e-3


def test_slope_percent(dem_path: Path):
    result = slope.handler(
        RasterFile(path=str(dem_path)), slope_format="percent"
    )
    arr = _open_array(result.path)
    valid = arr[arr != -9999.0]
    assert valid.min() >= 0


def test_slope_invalid_format(dem_path: Path):
    with pytest.raises(ValueError):
        slope.handler(RasterFile(path=str(dem_path)), slope_format="bogus")


def test_aspect(dem_path: Path):
    result = aspect.handler(RasterFile(path=str(dem_path)))
    assert Path(result.path).exists()


def test_color_relief(dem_path: Path, workdir: Path):
    ramp = workdir / "ramp.txt"
    ramp.write_text(
        "100 0 0 0\n"
        "150 255 255 255\n"
        "200 128 64 0\n"
    )
    result = color_relief.handler(
        RasterFile(path=str(dem_path)),
        File(path=str(ramp)),
    )
    ds = gdal.Open(result.path)
    # Color-relief default is RGB (3 bands).
    assert ds.RasterCount >= 3
    ds = None


def test_tri(dem_path: Path):
    result = tri.handler(RasterFile(path=str(dem_path)))
    assert Path(result.path).exists()


def test_tri_invalid_alg(dem_path: Path):
    with pytest.raises(ValueError):
        tri.handler(RasterFile(path=str(dem_path)), algorithm="Bogus")


def test_tpi(dem_path: Path):
    result = tpi.handler(RasterFile(path=str(dem_path)))
    assert Path(result.path).exists()


def test_roughness(dem_path: Path):
    result = roughness.handler(RasterFile(path=str(dem_path)))
    assert Path(result.path).exists()
