+++
title = "data.census_tiger"
description = "Download US Census TIGER/Line shapefiles."
weight = 8
+++

Download US Census TIGER/Line shapefiles for a given year and layer. National-scope layers ignore `state`; state-scoped layers require it.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `year` | number | TIGER/Line vintage year (2010 to current). |
| `layer` | string | Layer name, e.g. `states`, `counties`, `tracts`, `block_groups`, `blocks`, `zcta`, `roads`, `primary_roads`, `primary_secondary_roads`, `rails`, `places`, `urban_areas`, `cousub`. |
| `state` | string | State FIPS code (2 digits) or 2-letter code; required for state-scoped layers, ignored otherwise. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `vector` | directory (Shapefile) | Extracted shapefile directory (`.shp`, `.dbf`, `.shx`, `.prj`). |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000107
  name: data.census_tiger
  args:
    year: 2022
    layer: "counties"
    state: ""
```
