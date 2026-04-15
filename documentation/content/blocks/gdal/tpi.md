+++
title = "gdal.tpi"
description = "Compute the Topographic Position Index from a DEM (gdaldem TPI)."
weight = 26
+++

Compute the Topographic Position Index from a DEM.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdaldem TPI`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input DEM raster. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | TPI raster. |
