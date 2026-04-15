+++
title = "gdal.vector_difference"
description = "Geometric difference of two vector layers (A minus B)."
weight = 36
+++

Geometric difference `A \ B` — features in A with the overlap of B removed.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `ogr_layer_algebra Erase`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `a` | file (GeoJSON) | Base vector layer (A). |
| `b` | file (GeoJSON) | Vector layer whose geometry is removed from A (B). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vectors` | file (GeoJSON) | Difference `A \ B`. |
