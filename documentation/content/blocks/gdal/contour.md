+++
title = "gdal.contour"
description = "Generate contour lines from a raster (gdal_contour)."
weight = 12
+++

Generate contour lines from a raster (typically a DEM).

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdal_contour`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input raster (typically a DEM). |
| `band` | number | Band index (1-based). Default `1`. |
| `interval` | number | Contour interval in raster value units (default `1.0`). |
| `base` | number | Base contour elevation (default `0`). |
| `field_name` | string | Attribute name carrying the contour elevation. Default `"elev"`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vectors` | file (GeoJSON) | Contour lines with one attribute holding the elevation. |
