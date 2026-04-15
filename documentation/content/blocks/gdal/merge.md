+++
title = "gdal.merge"
description = "Mosaic multiple rasters into a single raster."
weight = 3
+++

Mosaic multiple rasters into a single raster.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdal_merge`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `sources` | collection of file (GeoTIFF) | Collection of input rasters to mosaic. |
| `resampling` | string | Resampling method used where sources overlap. Default `nearest`. |
| `output_format` | string | GDAL output driver short name (default `GTiff`). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Mosaiced raster. |
