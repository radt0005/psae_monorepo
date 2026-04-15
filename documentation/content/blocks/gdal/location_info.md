+++
title = "gdal.location_info"
description = "Query raster band values at a point (gdallocationinfo)."
weight = 29
+++

Read raster band values at a point.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdallocationinfo`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input raster. |
| `x` | number | Query X coordinate. |
| `y` | number | Query Y coordinate. |
| `coord_system` | string | `georef` (default) uses map coordinates; `pixel` uses pixel/line indices. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `values` | json | JSON object with the query point and per-band values. |
