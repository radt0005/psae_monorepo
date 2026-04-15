+++
title = "data.osm_extract_shp"
description = "Download OpenStreetMap extracts as Geofabrik shapefiles."
weight = 14
+++

Download an OpenStreetMap extract as Geofabrik's free shapefile bundle.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `region` | string | Geofabrik region slug, e.g. `north-america/us/oregon`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vector` | directory (Shapefile) | Extracted shapefile directory (layers ship as separate `.shp` files). |

## Example

```yaml
- id: 01890000-0000-0000-0000-00000000010d
  name: data.osm_extract_shp
  args:
    region: "north-america/us/maine"
```
