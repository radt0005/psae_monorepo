+++
title = "data.census_acs"
description = "Query the US Census ACS Data API and write the result as CSV."
weight = 7
+++

Query the US Census ACS (American Community Survey) Data API and write the result as CSV. The API requires a key; supply it via the `CENSUS_API_KEY` environment variable (pending proper secrets integration).

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `year` | number | ACS vintage year. |
| `dataset` | string | ACS dataset (`acs1` or `acs5`). |
| `table` | string | ACS table ID (e.g. `B01003`). |
| `geography` | string | Geography predicate (e.g. `"state:*"`, `"county:*&in=state:06"`). |
| `variables` | string | Optional comma-separated list of variable IDs. Defaults to `NAME,<table>_001E`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `table` | file (CSV) | Response rendered as CSV (first row = column headers). |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000106
  name: data.census_acs
  args:
    year: 2021
    dataset: "acs5"
    table: "B01003"
    geography: "state:*"
```
