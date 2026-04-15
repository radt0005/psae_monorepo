+++
title = "data.prism"
description = "Download PRISM climate rasters for a date range."
weight = 15
+++

Download PRISM climate normals / monthly / daily rasters for a date range at a given variable and cadence. Rasters are extracted as BIL (no GDAL dependency in this collection); convert downstream with `gdal.translate` if GeoTIFF is required.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `variable` | string | One of `ppt`, `tmean`, `tmin`, `tmax`, `tdmean`, `vpdmin`, `vpdmax`. |
| `start` | string | Start date as `YYYY-MM-DD`. |
| `end` | string | End date as `YYYY-MM-DD`. |
| `resolution` | string | `4km` or `800m` (`800m` is paywalled and will error cleanly). |
| `cadence` | string | `daily`, `monthly`, or `annual` (default `monthly`). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `rasters` | collection of file (GeoTIFF) | One raster per period within `[start, end]`. |

## Example

```yaml
- id: 01890000-0000-0000-0000-00000000010e
  name: data.prism
  args:
    variable: "tmean"
    start: "2023-01-01"
    end: "2023-12-01"
    resolution: "4km"
    cadence: "monthly"
```
