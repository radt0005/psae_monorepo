+++
title = "gdal.vector_intersection"
description = "Intersection of two vector layers (ogr_layer_algebra Intersection)."
weight = 35
+++

Intersection of two vector layers.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `ogr_layer_algebra Intersection`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `a` | file (GeoJSON) | First vector layer (A). |
| `b` | file (GeoJSON) | Second vector layer (B). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vectors` | file (GeoJSON) | Intersection of A and B. |
