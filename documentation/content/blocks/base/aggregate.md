+++
title = "base.aggregate"
description = "Compute aggregations over a table, optionally grouped."
weight = 3
+++

Compute aggregations (mean, median, sum, min, max, count, count_distinct, std, var, percentile, mode) over a table. With no `group_by` columns, returns a single-row table; with grouping columns, returns one row per group. Accepts CSV or Parquet input and emits Parquet output.

- **Kind:** `standard`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `table` | file (Parquet, CSV accepted) | Source table. |
| `aggregations` | string | JSON list of aggregation specs. Each spec is an object with keys `column`, `function` (`mean`, `median`, `mode`, `sum`, `min`, `max`, `count`, `count_distinct`, `std`, `var`, `percentile`), optional `p` (for `percentile`), and optional `as` (alias). |
| `group_by` | string | Comma-separated grouping columns. Empty (default) aggregates the whole table into a single row. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `result` | file (Parquet) | Aggregated table. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000003
  name: base.aggregate
  inputs:
    - 01890000-0000-0000-0000-000000000000
  args:
    aggregations: |
      [
        {"column": "score", "function": "mean", "as": "score_mean"},
        {"column": "score", "function": "percentile", "p": 0.95, "as": "score_p95"},
        {"column": "id", "function": "count", "as": "n"}
      ]
    group_by: "state"
```
