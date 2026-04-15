+++
title = "gdal.aspect"
description = "Compute aspect from a DEM (gdaldem aspect)."
weight = 23
+++

Compute aspect from a DEM.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdaldem aspect`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input DEM raster. |
| `zero_for_flat` | boolean | Write `0` for flat areas instead of `-9999`. Default `false`. |
| `trigonometric` | boolean | Return trigonometric angle (0=East) instead of azimuth (0=North). Default `false`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Aspect raster. |
