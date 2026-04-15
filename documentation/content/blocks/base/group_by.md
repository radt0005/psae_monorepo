+++
title = "base.group_by"
description = "Group a table by columns and compute aggregations per group."
weight = 4
+++

Group a table by one or more columns and compute aggregations per group. This is a convenience wrapper around [`base.aggregate`](/blocks/base/aggregate/) that promotes the grouping columns to a dedicated parameter so pipelines read more naturally.

- **Kind:** `standard`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `table` | file (Parquet, CSV accepted) | Source table. |
| `group_columns` | string | Comma-separated list of grouping columns (required, non-empty). |
| `aggregations` | string | JSON list of aggregation specs (same format as `base.aggregate`). |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `result` | file (Parquet) | Grouped table with one row per group. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000004
  name: base.group_by
  inputs:
    - 01890000-0000-0000-0000-000000000000
  args:
    group_columns: "state,county"
    aggregations: '[{"column": "population", "function": "sum", "as": "pop"}]'
```
