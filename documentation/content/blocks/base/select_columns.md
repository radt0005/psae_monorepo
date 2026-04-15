+++
title = "base.select_columns"
description = "Project a subset of columns from a table, keeping or dropping them."
weight = 2
+++

Project a subset of columns from a table. Use `mode: keep` (default) to retain the listed columns, or `mode: drop` to remove them. Accepts CSV or Parquet input and emits Parquet output.

- **Kind:** `standard`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `table` | file (Parquet, CSV accepted) | Source table. |
| `columns` | string | Comma-separated column names. |
| `mode` | string | Either `keep` (default) or `drop`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `result` | file (Parquet) | Projected table. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000002
  name: base.select_columns
  inputs:
    - 01890000-0000-0000-0000-000000000001
  args:
    columns: "id,name,score"
    mode: "keep"
```
