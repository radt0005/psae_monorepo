+++
title = "gdal.reduce_mosaic"
description = "Mosaic a collection of rasters back into one raster (fan-in after a map)."
weight = 46
+++

Mosaic a collection of rasters back into one raster (fan-in after a map).

- **Kind:** `reduce`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `tiles` | collection of file (GeoTIFF) | Rasters produced by the upstream mapped blocks. |
| `resampling` | string | Resampling method used where tiles overlap. Default `nearest`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Single mosaiced raster. |
