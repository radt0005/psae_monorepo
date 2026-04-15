+++
title = "gdal.viewshed"
description = "Compute a viewshed raster from an observer point (gdal_viewshed)."
weight = 18
+++

Compute a viewshed raster from an observer point.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdal_viewshed`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | DEM or height raster. |
| `observer_x` | number | Observer X coordinate in the DEM's CRS. |
| `observer_y` | number | Observer Y coordinate in the DEM's CRS. |
| `observer_height` | number | Observer height above the DEM surface (default `1.6` m). |
| `target_height` | number | Target height above the DEM surface (default `0.0` m). |
| `max_distance` | number | Maximum line-of-sight distance in CRS units. `0` = unlimited. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Binary viewshed raster (visible vs. hidden pixels). |
