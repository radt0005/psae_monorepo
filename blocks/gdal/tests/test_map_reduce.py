"""Tests for the map/reduce helper blocks (Phase 2.11)."""
from __future__ import annotations

from pathlib import Path

import pytest
import yaml
from osgeo import gdal
from spade import File, RasterFileCollection

from gdal_blocks import map_raster_tiles, reduce_mosaic, reduce_vrt


def test_map_raster_tiles_expansion(raster_collection: list[Path]):
    result = map_raster_tiles.handler(
        RasterFileCollection(paths=[str(p) for p in raster_collection])
    )
    assert isinstance(result, File)
    data = yaml.safe_load(Path(result.path).read_text())
    assert "items" in data
    assert len(data["items"]) == 3
    assert data["items"][0]["key"] == "tile_000"
    assert data["items"][0]["path"].endswith("tile_000.tif")


def test_map_raster_tiles_empty_rejected():
    with pytest.raises(ValueError):
        map_raster_tiles.handler(RasterFileCollection(paths=[]))


def test_reduce_mosaic(raster_collection: list[Path]):
    result = reduce_mosaic.handler(
        RasterFileCollection(paths=[str(p) for p in raster_collection])
    )
    ds = gdal.Open(result.path)
    assert ds is not None
    assert ds.RasterXSize >= 24  # three 8-wide tiles side by side.
    ds = None


def test_reduce_mosaic_empty_rejected():
    with pytest.raises(ValueError):
        reduce_mosaic.handler(RasterFileCollection(paths=[]))


def test_reduce_vrt(raster_collection: list[Path]):
    result = reduce_vrt.handler(
        RasterFileCollection(paths=[str(p) for p in raster_collection])
    )
    assert result.path.endswith(".vrt")
    ds = gdal.Open(result.path)
    assert ds is not None
    ds = None


def test_reduce_vrt_empty_rejected():
    with pytest.raises(ValueError):
        reduce_vrt.handler(RasterFileCollection(paths=[]))
