+++
title = "gdal.retile"
description = "Split a raster into a regular grid of smaller tiles (gdal_retile)."
weight = 9
+++

Split a raster into a regular grid of smaller tiles.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdal_retile`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input raster to retile. |
| `tile_width` | number | Tile width in pixels (default `256`). |
| `tile_height` | number | Tile height in pixels (default `256`). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `tiles` | collection of file (GeoTIFF) | Collection of tile rasters. |
