"""Shared fixtures for the gdal-blocks test suite.

Every test gets a fresh ``tmp_path`` working directory and chdirs into it,
so handlers can write relative-pathed outputs exactly as they would under
the Spade runtime.
"""
from __future__ import annotations

import json
import os
from pathlib import Path

import numpy as np
import pytest
from osgeo import gdal, ogr, osr


gdal.UseExceptions()
ogr.UseExceptions()
osr.UseExceptions()


# ---------------------------------------------------------------------------
# Environment

@pytest.fixture
def workdir(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """Change the working directory to a fresh tmp_path for the test."""
    monkeypatch.chdir(tmp_path)
    return tmp_path


# ---------------------------------------------------------------------------
# Raster fixtures

def _make_raster(
    path: Path,
    *,
    width: int = 16,
    height: int = 16,
    epsg: int = 32618,
    origin: tuple[float, float] = (500_000.0, 4_500_000.0),
    pixel_size: float = 30.0,
    pattern: str = "ramp",
    nodata: float | None = None,
    dtype: int = gdal.GDT_Float32,
) -> Path:
    """Write a small single-band GeoTIFF with a predictable pattern."""
    driver = gdal.GetDriverByName("GTiff")
    ds = driver.Create(str(path), width, height, 1, dtype)
    ds.SetGeoTransform((origin[0], pixel_size, 0.0, origin[1], 0.0, -pixel_size))
    srs = osr.SpatialReference()
    srs.ImportFromEPSG(epsg)
    ds.SetProjection(srs.ExportToWkt())

    if pattern == "ramp":
        y, x = np.mgrid[0:height, 0:width]
        data = (x + y).astype(np.float32)
    elif pattern == "dem":
        y, x = np.mgrid[0:height, 0:width]
        data = (100.0 + 0.5 * x + 2.0 * y).astype(np.float32)
    elif pattern == "constant":
        data = np.full((height, width), 7.0, dtype=np.float32)
    elif pattern == "noisy":
        rng = np.random.default_rng(42)
        data = rng.random((height, width)).astype(np.float32) * 100.0
    else:
        raise ValueError(f"unknown pattern {pattern!r}")

    band = ds.GetRasterBand(1)
    if nodata is not None:
        band.SetNoDataValue(nodata)
    band.WriteArray(data)
    band.FlushCache()
    ds.FlushCache()
    ds = None  # noqa: F841 — close
    return path


@pytest.fixture
def raster_path(workdir: Path) -> Path:
    """A small 16×16 GeoTIFF with a diagonal ramp pattern, EPSG:32618."""
    return _make_raster(workdir / "raster.tif", pattern="ramp")


@pytest.fixture
def dem_path(workdir: Path) -> Path:
    """A small 16×16 float32 DEM (for terrain analyses)."""
    return _make_raster(workdir / "dem.tif", pattern="dem")


@pytest.fixture
def raster_with_nodata(workdir: Path) -> Path:
    """A 16×16 raster with a nodata value of -9999 and a nodata pixel."""
    path = workdir / "raster_nodata.tif"
    _make_raster(path, pattern="ramp", nodata=-9999.0)
    # Poke a nodata pixel in the centre.
    ds = gdal.Open(str(path), gdal.GA_Update)
    band = ds.GetRasterBand(1)
    data = band.ReadAsArray()
    data[8, 8] = -9999.0
    band.WriteArray(data)
    band.FlushCache()
    ds = None  # noqa: F841
    return path


@pytest.fixture
def classified_raster(workdir: Path) -> Path:
    """A small integer-valued raster useful for polygonize/sieve tests."""
    path = workdir / "classified.tif"
    driver = gdal.GetDriverByName("GTiff")
    ds = driver.Create(str(path), 16, 16, 1, gdal.GDT_Int32)
    ds.SetGeoTransform((500_000.0, 30.0, 0.0, 4_500_000.0, 0.0, -30.0))
    srs = osr.SpatialReference()
    srs.ImportFromEPSG(32618)
    ds.SetProjection(srs.ExportToWkt())

    data = np.zeros((16, 16), dtype=np.int32)
    data[0:8, 0:8] = 1
    data[0:8, 8:16] = 2
    data[8:16, 0:8] = 3
    data[8:16, 8:16] = 4
    # One isolated single-pixel region, for sieve filtering tests.
    data[4, 4] = 99

    band = ds.GetRasterBand(1)
    band.WriteArray(data)
    band.FlushCache()
    ds = None  # noqa: F841
    return path


@pytest.fixture
def raster_collection(workdir: Path) -> list[Path]:
    """A collection of three small rasters with different offsets (for mosaic)."""
    paths = []
    for i in range(3):
        p = workdir / f"tile_{i:03d}.tif"
        _make_raster(
            p,
            width=8,
            height=8,
            origin=(500_000.0 + i * 240.0, 4_500_000.0),
            pattern="constant",
        )
        paths.append(p)
    return paths


# ---------------------------------------------------------------------------
# Vector fixtures

@pytest.fixture
def vector_path(workdir: Path) -> Path:
    """A GeoJSON with two polygons in EPSG:32618."""
    path = workdir / "vector.geojson"
    data = {
        "type": "FeatureCollection",
        "crs": {
            "type": "name",
            "properties": {"name": "urn:ogc:def:crs:EPSG::32618"},
        },
        "features": [
            {
                "type": "Feature",
                "properties": {"id": 1, "value": 10.0},
                "geometry": {
                    "type": "Polygon",
                    "coordinates": [[
                        [500_030.0, 4_499_910.0],
                        [500_150.0, 4_499_910.0],
                        [500_150.0, 4_499_790.0],
                        [500_030.0, 4_499_790.0],
                        [500_030.0, 4_499_910.0],
                    ]],
                },
            },
            {
                "type": "Feature",
                "properties": {"id": 2, "value": 20.0},
                "geometry": {
                    "type": "Polygon",
                    "coordinates": [[
                        [500_180.0, 4_499_770.0],
                        [500_330.0, 4_499_770.0],
                        [500_330.0, 4_499_640.0],
                        [500_180.0, 4_499_640.0],
                        [500_180.0, 4_499_770.0],
                    ]],
                },
            },
        ],
    }
    path.write_text(json.dumps(data))
    return path


@pytest.fixture
def vector_path_b(workdir: Path) -> Path:
    """A second GeoJSON (overlapping the first) for layer-algebra tests."""
    path = workdir / "vector_b.geojson"
    data = {
        "type": "FeatureCollection",
        "crs": {
            "type": "name",
            "properties": {"name": "urn:ogc:def:crs:EPSG::32618"},
        },
        "features": [
            {
                "type": "Feature",
                "properties": {"id": 10, "label": "a"},
                "geometry": {
                    "type": "Polygon",
                    "coordinates": [[
                        [500_080.0, 4_499_860.0],
                        [500_200.0, 4_499_860.0],
                        [500_200.0, 4_499_740.0],
                        [500_080.0, 4_499_740.0],
                        [500_080.0, 4_499_860.0],
                    ]],
                },
            },
        ],
    }
    path.write_text(json.dumps(data))
    return path


@pytest.fixture
def points_csv(workdir: Path) -> Path:
    """CSV of scattered points (for gdal.Grid tests)."""
    path = workdir / "points.csv"
    rows = ["x,y,z"]
    rng = np.random.default_rng(0)
    for _ in range(25):
        x = 500_000.0 + rng.random() * 480.0
        y = 4_499_520.0 + rng.random() * 480.0
        z = rng.random() * 10.0
        rows.append(f"{x},{y},{z}")
    path.write_text("\n".join(rows) + "\n")
    return path


# ---------------------------------------------------------------------------
# Spade-runtime simulation

@pytest.fixture
def invocation_dir(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """A fresh invocation directory with inputs/, outputs/, and chdir'd-in.

    Use this when you want to drive a block through ``spade.run`` rather
    than calling the handler directly.
    """
    (tmp_path / "inputs").mkdir()
    (tmp_path / "outputs").mkdir()
    monkeypatch.chdir(tmp_path)
    return tmp_path
