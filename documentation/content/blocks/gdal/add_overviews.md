+++
title = "gdal.add_overviews"
description = "Build internal pyramid overviews on a raster (gdaladdo)."
weight = 5
+++

Build internal pyramid overviews on a raster.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdaladdo`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input raster. A copy is made; the source is not modified. |
| `levels` | string | Space-separated decimation factors, e.g. `"2 4 8 16"`. Default `"2 4 8 16"`. |
| `resampling` | string | Resampling method for the overviews (default `average`). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Raster with overviews attached. |
