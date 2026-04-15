+++
title = "gdal.calc"
description = "Evaluate a NumPy expression over a raster (gdal_calc)."
weight = 13
+++

Evaluate a NumPy expression over a raster.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdal_calc`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input raster (exposed as variable `A` in the expression). |
| `expression` | string | NumPy expression using the variable `A`, e.g. `"A * 2 + 1"` or `"(A > 50) * 1"`. |
| `output_type` | string | Output data type (`Byte`, `Float32`, etc). Default `Float32`. |
| `nodata` | number | Output nodata value. Leave unset to skip. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Raster with the expression evaluated. |
