+++
title = "gdal.vector_clip"
description = "Clip vector layer A by the footprint of vector layer B."
weight = 39
+++

Clip vector layer A by the footprint of vector layer B.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `ogr_layer_algebra Clip`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `a` | file (GeoJSON) | Vector layer to clip (A). |
| `b` | file (GeoJSON) | Clip mask vector layer (B). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vectors` | file (GeoJSON) | Features of A clipped to B. |
