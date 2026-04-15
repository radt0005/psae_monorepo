+++
title = "data.ssurgo"
description = "Download USDA SSURGO soils data."
weight = 16
+++

Download USDA SSURGO soils data for a survey area or a whole state. Exactly one of `area` or `state` must be provided.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `area` | string | SSURGO survey area symbol (e.g. `CA077`). Mutually exclusive with `state`. |
| `state` | string | Two-letter state code for a full-state bulk download. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `dataset` | directory (FileGDB) | Extracted SSURGO FileGDB (or shapefile) directory. |

## Example

```yaml
- id: 01890000-0000-0000-0000-00000000010f
  name: data.ssurgo
  args:
    area: "ME001"
    state: ""
```
