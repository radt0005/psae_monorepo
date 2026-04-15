"""Tests for the raster information blocks (Phase 2.6)."""
from __future__ import annotations

import json
from pathlib import Path

import pytest
from spade import RasterFile

from gdal_blocks import compare, info, location_info, translate


def test_info_json(raster_path: Path):
    result = info.handler(RasterFile(path=str(raster_path)))
    data = json.loads(Path(result.path).read_text())
    assert data["size"] == [16, 16]
    assert "coordinateSystem" in data


def test_info_with_stats(raster_path: Path):
    result = info.handler(
        RasterFile(path=str(raster_path)), compute_stats=True
    )
    data = json.loads(Path(result.path).read_text())
    band = data["bands"][0]
    assert "minimum" in band
    assert "maximum" in band


def test_location_info_georef(raster_path: Path):
    from osgeo import gdal
    ds = gdal.Open(str(raster_path))
    gt = ds.GetGeoTransform()
    # Centre pixel.
    cx = gt[0] + gt[1] * 8.5
    cy = gt[3] + gt[5] * 8.5
    ds = None

    result = location_info.handler(
        RasterFile(path=str(raster_path)), x=cx, y=cy
    )
    data = json.loads(Path(result.path).read_text())
    assert "values" in data
    assert "1" in data["values"]


def test_location_info_pixel(raster_path: Path):
    result = location_info.handler(
        RasterFile(path=str(raster_path)),
        x=8,
        y=8,
        coord_system="pixel",
    )
    data = json.loads(Path(result.path).read_text())
    # Ramp pattern at (8, 8) is 8 + 8 = 16.
    assert data["values"]["1"] == pytest.approx(16.0)


def test_location_info_invalid_system(raster_path: Path):
    with pytest.raises(ValueError):
        location_info.handler(
            RasterFile(path=str(raster_path)),
            x=0, y=0,
            coord_system="bogus",
        )


def test_location_info_outside_bounds(raster_path: Path):
    with pytest.raises(ValueError):
        location_info.handler(
            RasterFile(path=str(raster_path)),
            x=1_000_000.0, y=1_000_000.0,
        )


def test_compare_identical(raster_path: Path):
    result = compare.handler(
        RasterFile(path=str(raster_path)),
        RasterFile(path=str(raster_path)),
    )
    data = json.loads(Path(result.path).read_text())
    assert data["match"] is True
    assert data["pixel_report"]["different_pixels"] == 0


def test_compare_different_sizes(raster_path: Path):
    resized = translate.handler(
        RasterFile(path=str(raster_path)), width=8, height=8
    )
    result = compare.handler(
        RasterFile(path=str(raster_path)),
        resized,
    )
    data = json.loads(Path(result.path).read_text())
    assert data["match"] is False
    assert any("size differs" in d for d in data["structural_differences"])
