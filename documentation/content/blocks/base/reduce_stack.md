+++
title = "base.reduce_stack"
description = "Concatenate a collection of tables row-wise."
weight = 11
+++

Concatenate a collection of tables row-wise. Equivalent to R's `rbind`, pandas' `concat(axis=0)`, or SQL `UNION ALL`. All tables must share a compatible schema; in non-strict mode, mismatched columns are unioned and missing values filled with null.

- **Kind:** `reduce`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `tables` | collection of file (Parquet) | Tables to stack. |
| `strict` | boolean | When `true`, require identical schemas across all tables. When `false` (default), diagonally-concatenate and fill missing columns with null. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `result` | file (Parquet) | Stacked table. |

## Example

```yaml
- id: 01890000-0000-0000-0000-00000000000b
  name: base.reduce_stack
  inputs:
    - 01890000-0000-0000-0000-000000000007
```
