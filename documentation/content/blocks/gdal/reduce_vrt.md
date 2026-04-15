+++
title = "gdal.reduce_vrt"
description = "Build a Virtual Raster from a collection of rasters (fan-in after a map)."
weight = 47
+++

Build a Virtual Raster from a collection of rasters (fan-in after a map).

- **Kind:** `reduce`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `tiles` | collection of file (GeoTIFF) | Rasters produced by the upstream mapped blocks. |
| `resolution` | string | One of `highest`, `lowest`, `average`, `user` (default `highest`). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vrt` | file (VRT) | Virtual raster referencing the tile collection. |
