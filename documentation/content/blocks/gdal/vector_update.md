+++
title = "gdal.vector_update"
description = "Update layer A with features from B (ogr_layer_algebra Update)."
weight = 41
+++

Update layer A with features from B.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `ogr_layer_algebra Update`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `a` | file (GeoJSON) | Base vector layer (A). |
| `b` | file (GeoJSON) | Vector layer whose features overwrite A (B). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vectors` | file (GeoJSON) | A updated with B. |
