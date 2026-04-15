+++
title = "data.fia"
description = "Download USFS Forest Inventory and Analysis (FIA) data."
weight = 18
+++

Download US Forest Service Forest Inventory and Analysis (FIA) public data from the USFS DataMart in CSV format. Defaults to the full national archive; a specific state can be selected.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `state` | string | Two-letter state/territory code or `all` for the entire country (default). |
| `tables` | string | Optional comma-separated list of FIA table names to extract (e.g. `"PLOT,TREE,COND"`). Leave blank to extract all tables from the archive. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `tables` | collection of file (CSV) | One CSV file per extracted FIA table. Filenames mirror the FIA naming convention (`<STATE>_<TABLE>.csv`). |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000111
  name: data.fia
  args:
    state: "ME"
    tables: "PLOT,TREE,COND"
```
