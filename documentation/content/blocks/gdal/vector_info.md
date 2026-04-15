+++
title = "gdal.vector_info"
description = "Report vector metadata as JSON (ogrinfo)."
weight = 42
+++

Report vector metadata as JSON.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `ogrinfo`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoJSON) | Input vector file. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `metadata` | json | Metadata report including layer count, feature count, schema, and extent. |
