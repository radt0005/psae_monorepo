+++
title = "gdal.vector_sym_difference"
description = "Symmetric difference of two vector layers (ogr_layer_algebra SymDifference)."
weight = 37
+++

Symmetric difference of two vector layers.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `ogr_layer_algebra SymDifference`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `a` | file (GeoJSON) | First vector layer (A). |
| `b` | file (GeoJSON) | Second vector layer (B). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vectors` | file (GeoJSON) | Symmetric difference `(A \ B) ∪ (B \ A)`. |
