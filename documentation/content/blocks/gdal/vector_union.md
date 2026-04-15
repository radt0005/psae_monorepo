+++
title = "gdal.vector_union"
description = "Union of two vector layers (ogr_layer_algebra Union)."
weight = 34
+++

Union of two vector layers.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `ogr_layer_algebra Union`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `a` | file (GeoJSON) | First vector layer (A). |
| `b` | file (GeoJSON) | Second vector layer (B). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vectors` | file (GeoJSON) | Union of A and B. |
