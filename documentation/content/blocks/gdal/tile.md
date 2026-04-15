+++
title = "gdal.tile"
description = "Generate XYZ/TMS tiles for web mapping (gdal2tiles)."
weight = 8
+++

Generate XYZ/TMS tiles for web mapping.

- **Kind:** `standard`
- **Network:** no
- **Wraps:** `gdal2tiles`

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | file (GeoTIFF) | Input raster to tile. |
| `zoom` | string | Zoom levels to generate, e.g. `"0-5"` or `"3-10"`. Default `"0-5"`. |
| `profile` | string | Tile profile. One of `mercator`, `geodetic`, `raster`. Default `mercator`. |
| `resampling` | string | Resampling method (default `average`). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `tiles` | directory | Directory of generated tiles in XYZ layout. |
