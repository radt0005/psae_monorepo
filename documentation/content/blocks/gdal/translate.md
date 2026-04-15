+++
title = "gdal.translate"
description = "Format conversion, subsetting, scaling (gdal_translate)."
weight = 1
+++

Convert raster format, subset, scale, or resize using `gdal_translate`.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdal_translate`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input raster. |
| `output_format` | string | GDAL output driver short name (default `GTiff`). Other common values: `COG`, `VRT`, `HFA`, `NetCDF`. |
| `output_type` | string | Output data type (`Byte`, `UInt16`, `Int16`, `UInt32`, `Int32`, `Float32`, `Float64`, `CInt16`, `CInt32`, `CFloat32`, `CFloat64`). Empty = preserve source. |
| `width` | number | Output width in pixels. `0` = preserve source width. |
| `height` | number | Output height in pixels. `0` = preserve source height. |
| `scale_min` | number | Source minimum for linear rescaling. Leave unset to skip rescaling. |
| `scale_max` | number | Source maximum for linear rescaling. Leave unset to skip rescaling. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Translated raster. |
