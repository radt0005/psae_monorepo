+++
title = "gdal.rasterize"
description = "Burn vector geometries into a raster (gdal_rasterize)."
weight = 10
+++

Burn vector geometries into a raster.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdal_rasterize`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `vectors` | file (GeoJSON) | Vector file whose geometries will be rasterized. |
| `reference` | file (GeoTIFF) | Reference raster whose extent, CRS, and pixel grid define the output. |
| `burn_value` | number | Constant value to burn into covered pixels (default `1`). |
| `attribute` | string | Optional attribute name; when set, pixel values come from this attribute instead of `burn_value`. |
| `all_touched` | boolean | Burn pixels touched by the geometry rather than centroid coverage. Default `false`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Rasterized output aligned to the reference raster. |
