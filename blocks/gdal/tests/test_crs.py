"""Tests for srs_info and transform_points (Phase 2.10)."""
from __future__ import annotations

import csv
import json
from pathlib import Path

import pytest
from spade import TabularFile

from gdal_blocks import srs_info, transform_points


def test_srs_info_epsg():
    result = srs_info.handler(crs="EPSG:4326")
    data = json.loads(Path(result.path).read_text())
    assert data["authority"] == "4326"
    assert data["is_geographic"] is True


def test_srs_info_empty_rejected():
    with pytest.raises(ValueError):
        srs_info.handler(crs="")


def test_srs_info_invalid_rejected():
    with pytest.raises(ValueError):
        srs_info.handler(crs="not-a-crs")


def test_transform_points_basic(workdir: Path):
    # Round-trip a single point through EPSG:4326 → EPSG:32618 → EPSG:4326.
    src = workdir / "pts.csv"
    src.write_text("x,y\n-73.0,41.0\n")
    result = transform_points.handler(
        TabularFile(path=str(src)),
        source_crs="EPSG:4326",
        target_crs="EPSG:32618",
    )

    with open(result.path) as f:
        rows = list(csv.reader(f))
    assert rows[0] == ["x", "y"]
    x, y = float(rows[1][0]), float(rows[1][1])
    # At 41°N, -73°E, UTM 18N yields easting ~668k, northing ~4.54M.
    assert 600_000 < x < 700_000
    assert 4_500_000 < y < 4_600_000


def test_transform_points_missing_column(workdir: Path):
    src = workdir / "pts.csv"
    src.write_text("longitude,latitude\n-73.0,41.0\n")
    with pytest.raises(ValueError):
        transform_points.handler(
            TabularFile(path=str(src)),
            source_crs="EPSG:4326",
            target_crs="EPSG:32618",
        )
