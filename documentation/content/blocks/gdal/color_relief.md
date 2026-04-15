+++
title = "gdal.color_relief"
description = "Apply a color ramp to a raster (gdaldem color-relief)."
weight = 24
+++

Apply a color ramp to a raster.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdaldem color-relief`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input raster (typically a DEM). |
| `color_ramp` | file | GDAL color text file. Each line is `"<value> R G B [A]"` with values matching the source's band values. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | 3- or 4-band RGB(A) raster. |
