+++
title = "gdal.slope"
description = "Compute slope from a DEM (gdaldem slope)."
weight = 22
+++

Compute slope from a DEM.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdaldem slope`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input DEM raster. |
| `scale` | number | Ratio of vertical units to horizontal units (default `1`). |
| `slope_format` | string | `degree` (default) or `percent`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Slope raster. |
