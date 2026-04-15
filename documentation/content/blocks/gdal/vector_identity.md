+++
title = "gdal.vector_identity"
description = "Identity overlay of two vector layers."
weight = 38
+++

Identity overlay — `A ∩ B` and `A \ B`, preserving A's attributes.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `ogr_layer_algebra Identity`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `a` | file (GeoJSON) | Base vector layer (A). |
| `b` | file (GeoJSON) | Method vector layer (B). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vectors` | file (GeoJSON) | Identity overlay of A with B. |
