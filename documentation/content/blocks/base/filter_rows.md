+++
title = "base.filter_rows"
description = "Filter rows of a table by a SQL WHERE-style predicate."
weight = 1
+++

Filter rows of a table by a SQL `WHERE`-style predicate. Accepts CSV or Parquet input and always emits Parquet output.

- **Kind:** `standard`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `table` | file (Parquet, CSV accepted) | Source table. |
| `expression` | string | SQL `WHERE`-style predicate, e.g. `"age > 30 AND state = 'NY'"`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `result` | file (Parquet) | Rows of the input table matching the predicate. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000001
  name: base.filter_rows
  inputs:
    - 01890000-0000-0000-0000-000000000000
  args:
    expression: "age > 18"
```
