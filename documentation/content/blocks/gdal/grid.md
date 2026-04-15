+++
title = "gdal.grid"
description = "Interpolate scattered points onto a regular raster grid (gdal_grid)."
weight = 17
+++

Interpolate scattered point data onto a regular raster grid.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdal_grid`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `points` | file (CSV) | CSV with columns `x`, `y`, `z`. |
| `algorithm` | string | Interpolation algorithm spec, e.g. `"invdist:power=2.0:radius1=0:radius2=0"`. Common: `invdist`, `average`, `nearest`, `linear`. Default `"invdist:power=2.0"`. |
| `width` | number | Output width in pixels (default `64`). |
| `height` | number | Output height in pixels (default `64`). |
| `z_field` | string | Column/attribute name carrying the value to grid (default `"z"`). |
| `x_field` | string | Column name for the X coordinate (default `"x"`). |
| `y_field` | string | Column name for the Y coordinate (default `"y"`). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Gridded raster. |
