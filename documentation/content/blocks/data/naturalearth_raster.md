+++
title = "data.naturalearth_raster"
description = "Download Natural Earth raster datasets."
weight = 9
+++

Download a [Natural Earth](https://www.naturalearthdata.com/) raster dataset at the requested scale and theme. Natural Earth only ships rasters at 10m and 50m scales.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `scale` | string | Scale — `10m` or `50m`. |
| `theme` | string | Theme identifier, e.g. `HYP_HR_SR_OB_DR`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Extracted raster (typically a GeoTIFF). |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000108
  name: data.naturalearth_raster
  args:
    scale: "10m"
    theme: "HYP_HR_SR_OB_DR"
```
