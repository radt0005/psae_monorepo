+++
title = "data.naturalearth_vector"
description = "Download Natural Earth vector datasets."
weight = 10
+++

Download a Natural Earth vector (cultural or physical) dataset at the requested scale and theme.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `scale` | string | Scale — one of `10m`, `50m`, `110m`. |
| `category` | string | Category — `cultural` or `physical`. |
| `theme` | string | Theme identifier, e.g. `admin_0_countries`, `rivers_lake_centerlines`, `populated_places`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vector` | directory (Shapefile) | Extracted shapefile directory. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000109
  name: data.naturalearth_vector
  args:
    scale: "110m"
    category: "cultural"
    theme: "admin_0_countries"
```
