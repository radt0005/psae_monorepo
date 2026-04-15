+++
title = "gdal.polygonize"
description = "Convert raster regions to polygon features (gdal_polygonize)."
weight = 11
+++

Convert raster regions of equal value into polygon features.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdal_polygonize`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input raster to polygonize. |
| `band` | number | Band index (1-based) to polygonize. Default `1`. |
| `field_name` | string | Attribute name for the pixel value on each output polygon. Default `"DN"`. |
| `connectedness` | number | Pixel connectedness, `4` (default) or `8`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vectors` | file (GeoJSON) | Polygon features with one attribute carrying the source pixel value. |
