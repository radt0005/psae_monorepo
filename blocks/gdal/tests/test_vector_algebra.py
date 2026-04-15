"""Tests for the vector layer-algebra blocks (Phase 2.8)."""
from __future__ import annotations

from pathlib import Path

from osgeo import ogr
from spade import VectorFile

from gdal_blocks import (
    vector_clip,
    vector_difference,
    vector_erase,
    vector_identity,
    vector_intersection,
    vector_sym_difference,
    vector_union,
    vector_update,
)


def _feature_count(path: str) -> int:
    ds = ogr.Open(path)
    count = ds.GetLayer(0).GetFeatureCount()
    ds = None
    return count


def _total_area(path: str) -> float:
    ds = ogr.Open(path)
    layer = ds.GetLayer(0)
    area = 0.0
    for feat in layer:
        geom = feat.GetGeometryRef()
        if geom:
            area += geom.GetArea()
    ds = None
    return area


def test_union(vector_path: Path, vector_path_b: Path):
    result = vector_union.handler(
        VectorFile(path=str(vector_path)),
        VectorFile(path=str(vector_path_b)),
    )
    assert Path(result.path).exists()
    assert _feature_count(result.path) > 0


def test_intersection(vector_path: Path, vector_path_b: Path):
    result = vector_intersection.handler(
        VectorFile(path=str(vector_path)),
        VectorFile(path=str(vector_path_b)),
    )
    # Fixture B overlaps both features of fixture A but produces a
    # strictly-smaller total area than A alone.
    assert 1 <= _feature_count(result.path) <= 2
    assert _total_area(result.path) < _total_area(str(vector_path))


def test_difference_removes_overlap(vector_path: Path, vector_path_b: Path):
    original = _total_area(str(vector_path))
    result = vector_difference.handler(
        VectorFile(path=str(vector_path)),
        VectorFile(path=str(vector_path_b)),
    )
    after = _total_area(result.path)
    assert after < original


def test_sym_difference(vector_path: Path, vector_path_b: Path):
    result = vector_sym_difference.handler(
        VectorFile(path=str(vector_path)),
        VectorFile(path=str(vector_path_b)),
    )
    assert Path(result.path).exists()
    assert _feature_count(result.path) > 0


def test_identity(vector_path: Path, vector_path_b: Path):
    result = vector_identity.handler(
        VectorFile(path=str(vector_path)),
        VectorFile(path=str(vector_path_b)),
    )
    assert Path(result.path).exists()


def test_clip(vector_path: Path, vector_path_b: Path):
    result = vector_clip.handler(
        VectorFile(path=str(vector_path)),
        VectorFile(path=str(vector_path_b)),
    )
    # Clipping by B retains only the part of A inside B.
    assert _total_area(result.path) < _total_area(str(vector_path))


def test_erase_matches_difference(vector_path: Path, vector_path_b: Path):
    diff = vector_difference.handler(
        VectorFile(path=str(vector_path)),
        VectorFile(path=str(vector_path_b)),
    )
    eras = vector_erase.handler(
        VectorFile(path=str(vector_path)),
        VectorFile(path=str(vector_path_b)),
    )
    assert _total_area(diff.path) == _total_area(eras.path)


def test_update(vector_path: Path, vector_path_b: Path):
    result = vector_update.handler(
        VectorFile(path=str(vector_path)),
        VectorFile(path=str(vector_path_b)),
    )
    assert Path(result.path).exists()
    # Update always produces at least as many features as A.
    assert _feature_count(result.path) >= _feature_count(str(vector_path))
