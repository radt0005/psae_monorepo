"""Tests for the vector I/O blocks (Phase 2.7)."""
from __future__ import annotations

from pathlib import Path

import pytest
from osgeo import ogr
from spade import VectorFile, VectorFileCollection

from gdal_blocks import vector_merge, vector_tile_index, vector_translate


# --------------------------------------------------------------------------- vector_translate

def test_vector_translate_to_geojson(vector_path: Path):
    result = vector_translate.handler(VectorFile(path=str(vector_path)))
    assert Path(result.path).exists()
    ds = ogr.Open(result.path)
    assert ds.GetLayer(0).GetFeatureCount() == 2


def test_vector_translate_to_gpkg(vector_path: Path):
    result = vector_translate.handler(
        VectorFile(path=str(vector_path)),
        output_format="GPKG",
    )
    assert result.path.endswith(".gpkg")
    ds = ogr.Open(result.path)
    assert ds.GetLayer(0).GetFeatureCount() == 2


def test_vector_translate_with_where(vector_path: Path):
    result = vector_translate.handler(
        VectorFile(path=str(vector_path)),
        where="id = 1",
    )
    ds = ogr.Open(result.path)
    assert ds.GetLayer(0).GetFeatureCount() == 1


def test_vector_translate_reproject(vector_path: Path):
    result = vector_translate.handler(
        VectorFile(path=str(vector_path)),
        target_crs="EPSG:4326",
    )
    ds = ogr.Open(result.path)
    srs = ds.GetLayer(0).GetSpatialRef()
    assert "4326" in srs.ExportToWkt() or "WGS 84" in srs.ExportToWkt()


# --------------------------------------------------------------------------- vector_merge

def test_vector_merge(vector_path: Path, vector_path_b: Path):
    result = vector_merge.handler(
        VectorFileCollection(paths=[str(vector_path), str(vector_path_b)])
    )
    ds = ogr.Open(result.path)
    # 2 features from first + 1 from second = 3.
    assert ds.GetLayer(0).GetFeatureCount() == 3


def test_vector_merge_empty_rejected():
    with pytest.raises(ValueError):
        vector_merge.handler(VectorFileCollection(paths=[]))


# --------------------------------------------------------------------------- vector_tile_index

def test_vector_tile_index(vector_path: Path, vector_path_b: Path):
    result = vector_tile_index.handler(
        VectorFileCollection(paths=[str(vector_path), str(vector_path_b)])
    )
    ds = ogr.Open(result.path)
    layer = ds.GetLayer(0)
    assert layer.GetFeatureCount() == 2
    feat = layer.GetNextFeature()
    assert feat.GetField("location")


def test_vector_tile_index_empty_rejected():
    with pytest.raises(ValueError):
        vector_tile_index.handler(VectorFileCollection(paths=[]))
