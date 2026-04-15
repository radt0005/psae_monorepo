+++
title = "base.reduce_join"
description = "Join a collection of tables left-to-right on one or more key columns."
weight = 12
+++

Join a collection of tables on one or more key columns (the SQL `JOIN` analogue). Tables are joined sequentially in the collection's lexicographic filename order, accumulating left-to-right.

- **Kind:** `reduce`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `tables` | collection of file (Parquet) | Tables to join, processed in lexicographic order of filename. |
| `on` | string | Comma-separated list of key column names. |
| `how` | string | One of `inner` (default), `left`, `right`, `outer`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `result` | file (Parquet) | Joined table. |

## Example

```yaml
- id: 01890000-0000-0000-0000-00000000000c
  name: base.reduce_join
  inputs:
    - 01890000-0000-0000-0000-000000000007
  args:
    on: "id"
    how: "inner"
```
