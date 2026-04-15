+++
title = "gdal.tile_index"
description = "Build a vector tile index for a collection of rasters (gdaltindex)."
weight = 6
+++

Build a vector tile index (footprint polygons) for a collection of rasters.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdaltindex`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `sources` | collection of file (GeoTIFF) | Collection of rasters to index. |
| `location_field` | string | Attribute name to store the source raster path (default `location`). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `index` | file (GeoJSON) | Vector index of raster footprints. |
