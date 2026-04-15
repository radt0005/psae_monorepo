+++
title = "gdal.roughness"
description = "Compute terrain roughness from a DEM (gdaldem roughness)."
weight = 27
+++

Compute terrain roughness from a DEM.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdaldem roughness`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input DEM raster. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Roughness raster. |
