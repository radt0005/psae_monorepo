+++
title = "gdal.warp"
description = "Reproject, resample, or warp a raster (gdalwarp)."
weight = 2
+++

Reproject, resample, or warp a raster using `gdalwarp`.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdalwarp`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input raster. |
| `target_crs` | string | Target CRS (e.g. `"EPSG:4326"`, WKT, or PROJ string). Empty = preserve source CRS. |
| `resolution` | number | Target pixel size in target CRS units. `0` = preserve source resolution. |
| `resampling` | string | Resampling method. One of `nearest`, `bilinear`, `cubic`, `cubicspline`, `lanczos`, `average`, `mode`, `max`, `min`, `med`, `q1`, `q3`, `sum`, `rms`. Default `nearest`. |
| `output_format` | string | GDAL output driver short name (default `GTiff`). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Warped raster. |
