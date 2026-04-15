+++
title = "gdal.clip_raster_by_extent"
description = "Clip a raster to a bounding box in its source CRS."
weight = 20
+++

Clip a raster to a bounding box in its source CRS.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdalwarp` (bbox)

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Raster to clip. |
| `xmin` | number | Minimum X (source CRS units). |
| `ymin` | number | Minimum Y (source CRS units). |
| `xmax` | number | Maximum X (source CRS units). |
| `ymax` | number | Maximum Y (source CRS units). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Clipped raster. |
