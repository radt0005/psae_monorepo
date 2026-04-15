"""Shared helpers for the gdal block collection."""
from __future__ import annotations

from pathlib import Path

from osgeo import gdal, ogr, osr


gdal.UseExceptions()
ogr.UseExceptions()
osr.UseExceptions()


def ensure_parent(path: str | Path) -> str:
    """Ensure the parent directory of *path* exists; return *path* as str."""
    p = Path(path)
    p.parent.mkdir(parents=True, exist_ok=True)
    return str(p)


def output_path(name: str, ext: str) -> str:
    """Build a stable output path ``<name>.<ext>`` in the current working dir."""
    ext = ext.lstrip(".")
    return str(Path(f"{name}.{ext}"))


def srs_from_user(value) -> osr.SpatialReference:
    """Parse a user-supplied CRS (e.g. ``"EPSG:4326"``, WKT, PROJ) into an SRS."""
    srs = osr.SpatialReference()
    if isinstance(value, int):
        srs.ImportFromEPSG(value)
        return srs
    if isinstance(value, str):
        try:
            if srs.SetFromUserInput(value) == 0:
                return srs
        except RuntimeError:
            pass
    raise ValueError(f"unrecognized CRS value: {value!r}")
