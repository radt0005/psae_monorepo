+++
title = "gdal.proximity"
description = "Compute a distance-to-features raster (gdal_proximity)."
weight = 16
+++

Compute a distance-to-features raster.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdal_proximity`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Source raster whose non-zero / target-value pixels are the "features". |
| `target_values` | string | Comma-separated list of target pixel values. Empty = non-zero pixels. |
| `distance_units` | string | `pixel` (default) or `georef` (projection units). |
| `max_distance` | number | Maximum distance to report. `0` = unlimited. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Float32 raster of distances to the nearest feature. |
