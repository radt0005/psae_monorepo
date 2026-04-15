+++
title = "gdal.info"
description = "Report raster metadata as JSON (gdalinfo)."
weight = 28
+++

Report raster metadata as JSON.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdalinfo`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input raster. |
| `compute_stats` | boolean | Compute per-band statistics (min/max/mean/stddev). Default `false`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `metadata` | json | Raster metadata including CRS, bounds, bands, data type, and nodata. |
