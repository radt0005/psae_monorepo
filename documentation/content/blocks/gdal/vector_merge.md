+++
title = "gdal.vector_merge"
description = "Merge a collection of vector files into a single layer (ogrmerge)."
weight = 32
+++

Merge a collection of vector files into a single layer.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `ogrmerge`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `sources` | collection of file (GeoJSON) | Collection of vector files to merge. |
| `output_format` | string | OGR output driver short name. Default `GeoJSON`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vectors` | file (GeoJSON) | Merged vector file. |
