+++
title = "data.osm_extract_pbf"
description = "Download OpenStreetMap extracts in .osm.pbf format."
weight = 13
+++

Download an OpenStreetMap extract in `.osm.pbf` format from [Geofabrik](https://download.geofabrik.de/).

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `region` | string | Geofabrik region slug, e.g. `north-america/us/oregon` or `europe/germany`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `file` | file (OSMPBF) | OpenStreetMap protocol-buffer binary file (`.osm.pbf`). |

## Example

```yaml
- id: 01890000-0000-0000-0000-00000000010c
  name: data.osm_extract_pbf
  args:
    region: "north-america/us/maine"
```
