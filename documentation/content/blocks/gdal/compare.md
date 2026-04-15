+++
title = "gdal.compare"
description = "Compare two rasters and report differences as JSON (gdalcompare)."
weight = 30
+++

Compare two rasters and report differences as JSON.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdalcompare`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `golden` | file (GeoTIFF) | Reference raster. |
| `new` | file (GeoTIFF) | Raster to compare against the reference. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `report` | json | JSON report of structural and pixel-level differences. |
