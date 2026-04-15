+++
title = "gdal.clip_raster_by_vector"
description = "Clip a raster to a vector boundary (gdalwarp -cutline)."
weight = 19
+++

Clip a raster to a vector boundary.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdalwarp -cutline`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Raster to clip. |
| `boundary` | file (GeoJSON) | Vector boundary defining the clip region. |
| `crop_to_cutline` | boolean | Shrink the output extent to the cutline bounding box. Default `true`. |
| `all_touched` | boolean | Include pixels touched by the cutline rather than centroid coverage. Default `false`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Clipped raster. |
