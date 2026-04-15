+++
title = "gdal.vector_translate"
description = "Convert vector format, reproject, or filter (ogr2ogr)."
weight = 31
+++

Convert vector format, reproject, or filter features.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `ogr2ogr`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoJSON) | Input vector file. |
| `output_format` | string | OGR output driver short name. Default `GeoJSON`. |
| `target_crs` | string | Target CRS (e.g. `"EPSG:4326"`). Empty = preserve source CRS. |
| `where` | string | SQL WHERE clause to filter features. Empty = no filter. |
| `sql` | string | OGR SQL statement to run on the source. Overrides `where` when set. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vectors` | file (GeoJSON) | Translated vector file. |
