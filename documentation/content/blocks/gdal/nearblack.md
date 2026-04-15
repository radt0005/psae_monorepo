+++
title = "gdal.nearblack"
description = "Clean near-black/white pixels at raster edges."
weight = 7
+++

Convert near-black/white pixels at raster edges to exact black/white or nodata.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `nearblack`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input raster. |
| `near` | number | Tolerance around the target color (default `15`). |
| `white` | boolean | Target near-white instead of near-black pixels. Default `false`. |
| `set_alpha` | boolean | Add an alpha band where collar pixels become transparent. Default `false`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Raster with edge pixels cleaned up. |
