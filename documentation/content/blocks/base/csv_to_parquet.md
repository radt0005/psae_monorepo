+++
title = "base.csv_to_parquet"
description = "Convert a CSV file to Parquet."
weight = 5
+++

Convert a CSV file to Parquet. Useful as an explicit conversion step at pipeline boundaries; tabular blocks accept CSV directly, but converting once up front avoids re-parsing at every step.

- **Kind:** `standard`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `table` | file (CSV) | Source CSV file. |
| `delimiter` | string | Field delimiter (single ASCII character). Defaults to `,`. |
| `has_header` | boolean | Whether the input CSV has a header row. Defaults to `true`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `result` | file (Parquet) | Parquet encoding of the input CSV. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000005
  name: base.csv_to_parquet
  inputs:
    - 01890000-0000-0000-0000-000000000000
```
