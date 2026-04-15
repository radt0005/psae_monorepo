"""Shared resampling-method enum and translation helpers."""
from __future__ import annotations

from osgeo import gdal, gdalconst


_GDALWARP_METHODS = {
    "nearest": "near",
    "near": "near",
    "bilinear": "bilinear",
    "cubic": "cubic",
    "cubicspline": "cubicspline",
    "lanczos": "lanczos",
    "average": "average",
    "rms": "rms",
    "mode": "mode",
    "max": "max",
    "min": "min",
    "med": "med",
    "q1": "q1",
    "q3": "q3",
    "sum": "sum",
}

_GDALCONST_METHODS = {
    "nearest": gdalconst.GRA_NearestNeighbour,
    "near": gdalconst.GRA_NearestNeighbour,
    "bilinear": gdalconst.GRA_Bilinear,
    "cubic": gdalconst.GRA_Cubic,
    "cubicspline": gdalconst.GRA_CubicSpline,
    "lanczos": gdalconst.GRA_Lanczos,
    "average": gdalconst.GRA_Average,
    "mode": gdalconst.GRA_Mode,
    "max": gdalconst.GRA_Max,
    "min": gdalconst.GRA_Min,
    "med": gdalconst.GRA_Med,
    "q1": gdalconst.GRA_Q1,
    "q3": gdalconst.GRA_Q3,
    "sum": gdalconst.GRA_Sum,
}


def warp_method(name: str) -> str:
    """Return the gdalwarp-style resampling method string."""
    key = (name or "nearest").lower()
    if key not in _GDALWARP_METHODS:
        raise ValueError(
            f"unknown resampling method {name!r}; "
            f"expected one of {sorted(_GDALWARP_METHODS)}"
        )
    return _GDALWARP_METHODS[key]


def gra_constant(name: str) -> int:
    """Return the GDAL resample-algorithm constant (``gdalconst.GRA_*``)."""
    key = (name or "nearest").lower()
    if key not in _GDALCONST_METHODS:
        raise ValueError(
            f"unknown resampling method {name!r}; "
            f"expected one of {sorted(_GDALCONST_METHODS)}"
        )
    return _GDALCONST_METHODS[key]
