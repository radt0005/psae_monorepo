+++
title = "gdal.tri"
description = "Compute the Terrain Ruggedness Index from a DEM (gdaldem TRI)."
weight = 25
+++

Compute the Terrain Ruggedness Index from a DEM.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdaldem TRI`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input DEM raster. |
| `algorithm` | string | TRI algorithm. `Wilson` (default) or `Riley`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | TRI raster. |
