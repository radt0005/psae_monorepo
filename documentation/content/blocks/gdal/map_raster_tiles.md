+++
title = "gdal.map_raster_tiles"
description = "Fan out over a collection of raster tiles for per-tile processing."
weight = 45
+++

Enumerate a raster collection for per-tile fan-out. Downstream blocks run once per tile in parallel.

- **Kind:** `map`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `sources` | collection of file (GeoTIFF) | Collection of raster tiles to fan out over. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `manifest` | expansion | Expansion manifest with one item per input tile. |

See [Map/Reduce](/concepts/map-reduce/) for the full fan-out pattern.
