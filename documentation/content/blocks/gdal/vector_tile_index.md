+++
title = "gdal.vector_tile_index"
description = "Build a vector footprint index for a collection of vector files (ogrtindex)."
weight = 33
+++

Build a vector footprint index for a collection of vector files.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `ogrtindex`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `sources` | collection of file (GeoJSON) | Collection of vector files to index. |
| `location_field` | string | Attribute name storing the source path (default `location`). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `index` | file (GeoJSON) | Per-source extent polygons with a path attribute. |
