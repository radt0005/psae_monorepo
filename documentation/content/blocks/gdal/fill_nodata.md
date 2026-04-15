+++
title = "gdal.fill_nodata"
description = "Interpolate nodata pixels from surrounding neighbours (gdal_fillnodata)."
weight = 15
+++

Interpolate missing-data pixels from surrounding neighbours.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdal_fillnodata`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Raster with nodata pixels to fill. |
| `max_distance` | number | Maximum search distance in pixels (default `100`). |
| `smoothing_iterations` | number | Number of 3x3 smoothing iterations applied after fill (default `0`). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Raster with nodata filled. |
