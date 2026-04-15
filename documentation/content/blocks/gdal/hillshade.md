+++
title = "gdal.hillshade"
description = "Generate a hillshade raster from a DEM (gdaldem hillshade)."
weight = 21
+++

Generate a hillshade raster from a DEM.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdaldem hillshade`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input DEM raster. |
| `azimuth` | number | Azimuth of the light source in degrees (default `315`). |
| `altitude` | number | Altitude of the light source in degrees (default `45`). |
| `z_factor` | number | Vertical exaggeration applied to elevations (default `1`). |
| `scale` | number | Ratio of vertical units to horizontal units (default `1`). |
| `multidirectional` | boolean | Use multi-directional shading. Default `false`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Hillshade raster. |
