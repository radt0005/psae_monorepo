"""Tests for the raster I/O blocks (Phase 2.1)."""
from __future__ import annotations

from pathlib import Path

import pytest
from osgeo import gdal, ogr
from spade import (
    Directory,
    RasterFile,
    RasterFileCollection,
    VectorFile,
)

from gdal_blocks import (
    add_overviews,
    build_vrt,
    merge,
    nearblack,
    retile,
    tile,
    tile_index,
    translate,
    warp,
)


# --------------------------------------------------------------------------- translate

def test_translate_basic(raster_path: Path):
    result = translate.handler(RasterFile(path=str(raster_path)))
    assert isinstance(result, RasterFile)
    assert Path(result.path).exists()
    ds = gdal.Open(result.path)
    assert ds.RasterXSize == 16


def test_translate_resize(raster_path: Path):
    result = translate.handler(
        RasterFile(path=str(raster_path)), width=8, height=8
    )
    ds = gdal.Open(result.path)
    assert (ds.RasterXSize, ds.RasterYSize) == (8, 8)


def test_translate_output_type(raster_path: Path):
    result = translate.handler(
        RasterFile(path=str(raster_path)), output_type="Byte"
    )
    ds = gdal.Open(result.path)
    assert ds.GetRasterBand(1).DataType == gdal.GDT_Byte


# --------------------------------------------------------------------------- warp

def test_warp_reproject(raster_path: Path):
    result = warp.handler(
        RasterFile(path=str(raster_path)), target_crs="EPSG:4326"
    )
    assert Path(result.path).exists()
    ds = gdal.Open(result.path)
    proj = ds.GetProjection()
    assert "4326" in proj or "WGS 84" in proj


def test_warp_resolution(raster_path: Path):
    result = warp.handler(
        RasterFile(path=str(raster_path)), resolution=60.0
    )
    ds = gdal.Open(result.path)
    gt = ds.GetGeoTransform()
    assert gt[1] == 60.0
    assert abs(gt[5]) == 60.0


def test_warp_invalid_resampling(raster_path: Path):
    with pytest.raises(ValueError):
        warp.handler(
            RasterFile(path=str(raster_path)), resampling="nonsense"
        )


# --------------------------------------------------------------------------- merge

def test_merge_basic(raster_collection: list[Path]):
    result = merge.handler(
        RasterFileCollection(paths=[str(p) for p in raster_collection])
    )
    assert isinstance(result, RasterFile)
    ds = gdal.Open(result.path)
    # Three 8-wide tiles at 30m pixels side by side => 24 px wide at least.
    assert ds.RasterXSize >= 24


def test_merge_empty_rejected():
    with pytest.raises(ValueError):
        merge.handler(RasterFileCollection(paths=[]))


# --------------------------------------------------------------------------- build_vrt

def test_build_vrt_basic(raster_collection: list[Path]):
    result = build_vrt.handler(
        RasterFileCollection(paths=[str(p) for p in raster_collection])
    )
    assert Path(result.path).exists()
    assert result.path.endswith(".vrt")
    ds = gdal.Open(result.path)
    assert ds is not None


def test_build_vrt_separate_bands(raster_collection: list[Path]):
    result = build_vrt.handler(
        RasterFileCollection(paths=[str(p) for p in raster_collection]),
        separate=True,
    )
    ds = gdal.Open(result.path)
    assert ds.RasterCount == 3


# --------------------------------------------------------------------------- add_overviews

def test_add_overviews_basic(raster_path: Path):
    # The ramp raster is 16x16; use small factors so overviews are computable.
    result = add_overviews.handler(
        RasterFile(path=str(raster_path)), levels="2 4"
    )
    ds = gdal.Open(result.path)
    band = ds.GetRasterBand(1)
    assert band.GetOverviewCount() == 2


def test_add_overviews_empty_levels_rejected(raster_path: Path):
    with pytest.raises(ValueError):
        add_overviews.handler(
            RasterFile(path=str(raster_path)), levels=""
        )


# --------------------------------------------------------------------------- tile_index

def test_tile_index_basic(raster_collection: list[Path]):
    result = tile_index.handler(
        RasterFileCollection(paths=[str(p) for p in raster_collection])
    )
    assert isinstance(result, VectorFile)
    ds = ogr.Open(result.path)
    layer = ds.GetLayer(0)
    assert layer.GetFeatureCount() == 3
    # Every feature should have a location attribute.
    feat = layer.GetNextFeature()
    assert feat.GetField("location")


def test_tile_index_empty_rejected():
    with pytest.raises(ValueError):
        tile_index.handler(RasterFileCollection(paths=[]))


# --------------------------------------------------------------------------- nearblack

def test_nearblack_basic(workdir: Path):
    # Build a 3-band Byte raster (expected input shape for nearblack).
    path = workdir / "rgb.tif"
    driver = gdal.GetDriverByName("GTiff")
    ds = driver.Create(str(path), 16, 16, 3, gdal.GDT_Byte)
    ds.SetGeoTransform((0, 1, 0, 0, 0, -1))
    for band_idx in range(1, 4):
        band = ds.GetRasterBand(band_idx)
        arr = band.ReadAsArray()
        arr[:] = 200
        arr[0, :] = 2   # near-black edge row
        band.WriteArray(arr)
    ds.FlushCache()
    ds = None

    result = nearblack.handler(RasterFile(path=str(path)))
    assert Path(result.path).exists()


# --------------------------------------------------------------------------- tile

def test_tile_basic(raster_path: Path):
    # Convert to Byte first — gdal2tiles requires 8-bit.
    byte = translate.handler(
        RasterFile(path=str(raster_path)), output_type="Byte"
    )
    # Our 16x16 fixture covers ~480m at UTM 18N; use Web Mercator zooms
    # high enough that at least one tile lands on the footprint.
    result = tile.handler(byte, zoom="14-15")
    assert isinstance(result, Directory)
    assert Path(result.path).is_dir()
    contents = list(Path(result.path).rglob("*.png"))
    assert contents, "gdal2tiles produced no tiles"


# --------------------------------------------------------------------------- retile

def test_retile_basic(raster_path: Path):
    result = retile.handler(
        RasterFile(path=str(raster_path)),
        tile_width=8,
        tile_height=8,
    )
    assert isinstance(result, RasterFileCollection)
    # 16x16 with 8x8 tiles => 4 tiles.
    assert len(result.paths) == 4
    for p in result.paths:
        ds = gdal.Open(p)
        assert ds.RasterXSize == 8
        assert ds.RasterYSize == 8
