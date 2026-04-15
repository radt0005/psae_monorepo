+++
title = "data.nhd"
description = "Download USGS National Hydrography Dataset data by watershed."
weight = 11
+++

Download USGS National Hydrography Dataset (NHD) data for a given HUC-4 or HUC-8 watershed.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `huc` | string | HUC-4 or HUC-8 watershed code (numeric, 4 or 8 digits). |
| `resolution` | string | One of `medium` or `high` (default). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `dataset` | directory (GeoPackage) | NHD GeoPackage (or FileGDB) extracted to a directory. |

## Example

```yaml
- id: 01890000-0000-0000-0000-00000000010a
  name: data.nhd
  args:
    huc: "0105"
    resolution: "high"
```
