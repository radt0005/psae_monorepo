+++
title = "gdal.transform_points"
description = "Transform a tabular file of coordinates between two CRSs (gdaltransform)."
weight = 44
+++

Transform a tabular file of coordinates between two CRSs.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdaltransform`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `points` | file (CSV) | Input CSV with `x`, `y` (and optional `z`) columns. |
| `source_crs` | string | Source CRS (e.g. `"EPSG:4326"`). |
| `target_crs` | string | Target CRS (e.g. `"EPSG:32618"`). |
| `x_field` | string | Name of the X column in the CSV (default `"x"`). |
| `y_field` | string | Name of the Y column in the CSV (default `"y"`). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `points` | file (CSV) | Output CSV with columns `x`, `y` and (when present) `z` in the target CRS. |
