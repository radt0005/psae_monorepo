+++
title = "gdal.build_vrt"
description = "Build a GDAL Virtual Raster from a collection of rasters."
weight = 4
+++

Build a GDAL Virtual Raster (VRT) from a collection of rasters.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdalbuildvrt`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `sources` | collection of file (GeoTIFF) | Collection of input rasters to assemble into the VRT. |
| `resolution` | string | One of `highest`, `lowest`, `average`, `user` (default `highest`). |
| `separate` | boolean | Stack sources as separate bands instead of a mosaic. Default `false`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vrt` | file (VRT) | Virtual raster referencing the input collection. |
