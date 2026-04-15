+++
title = "gdal.vector_erase"
description = "Erase features of A by B (alias of vector_difference)."
weight = 40
+++

Erase features of A by B. Alias of [`gdal.vector_difference`](/blocks/gdal/vector_difference/).

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `ogr_layer_algebra Erase`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `a` | file (GeoJSON) | Base vector layer (A). |
| `b` | file (GeoJSON) | Vector layer whose geometry is erased from A (B). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vectors` | file (GeoJSON) | A with B erased. |
