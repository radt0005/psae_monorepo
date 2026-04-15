+++
title = "data.usgs_3dep"
description = "Fetch USGS 3DEP elevation tiles overlapping an AOI."
weight = 17
+++

Fetch USGS 3D Elevation Program (3DEP) raster tiles overlapping an area of interest. Returns a collection of tiles; compose downstream with `gdal.merge` or `gdal.build_vrt` if a single raster is needed.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `aoi` | file (GeoJSON) | Area of interest polygon (GeoJSON FeatureCollection or Feature). |
| `resolution` | string | One of `1m`, `10m`, `30m`, `60m`. |
| `product` | string | 3DEP product, e.g. `DEM` (default). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `rasters` | collection of file (GeoTIFF) | Collection of 3DEP tile rasters overlapping the AOI. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000110
  name: data.usgs_3dep
  inputs:
    - 01890000-0000-0000-0000-000000000000
  args:
    resolution: "10m"
    product: "DEM"
```
