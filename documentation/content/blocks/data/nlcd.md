+++
title = "data.nlcd"
description = "Download National Land Cover Database rasters."
weight = 12
+++

Download a National Land Cover Database (NLCD) raster product. Full CONUS rasters are very large (> 5 GB); expect to clip with a downstream `gdal.clip_raster_by_vector` block in most pipelines.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `year` | number | NLCD vintage year (2001, 2004, 2006, 2008, 2011, 2013, 2016, 2019, 2021). |
| `product` | string | One of `land_cover`, `impervious`, `canopy`. |
| `region` | string | One of `CONUS` (default), `AK`, `HI`, `PR`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | NLCD product as a single GeoTIFF (or whatever MRLC ships). |

## Example

```yaml
- id: 01890000-0000-0000-0000-00000000010b
  name: data.nlcd
  args:
    year: 2021
    product: "land_cover"
    region: "CONUS"
```
