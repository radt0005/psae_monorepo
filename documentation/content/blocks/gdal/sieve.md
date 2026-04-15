+++
title = "gdal.sieve"
description = "Remove small connected regions from a raster (gdal_sieve)."
weight = 14
+++

Remove connected raster regions smaller than a threshold.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdal_sieve`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Categorical or integer raster to sieve. |
| `threshold` | number | Minimum region size in pixels. Regions smaller than this are replaced by a neighbour value. |
| `connectedness` | number | Pixel connectedness, `4` (default) or `8`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `raster` | file (GeoTIFF) | Sieved raster. |
