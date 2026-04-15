+++
title = "gdal.srs_info"
description = "Report CRS metadata in multiple forms (gdalsrsinfo)."
weight = 43
+++

Report CRS metadata in multiple forms.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdalsrsinfo`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `crs` | string | CRS value — e.g. `"EPSG:4326"`, a WKT string, or a PROJ string. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `metadata` | json | CRS metadata including WKT, PROJ, EPSG authority, and axis info. |
