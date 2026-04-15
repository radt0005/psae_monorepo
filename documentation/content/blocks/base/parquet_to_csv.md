+++
title = "base.parquet_to_csv"
description = "Convert a Parquet file to CSV."
weight = 6
+++

Convert a Parquet file to CSV. Useful at pipeline boundaries when downstream consumers (humans, spreadsheets, legacy tools) need CSV.

- **Kind:** `standard`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `table` | file (Parquet) | Source Parquet file. |
| `delimiter` | string | Field delimiter (single ASCII character). Defaults to `,`. |
| `include_header` | boolean | Whether to emit a header row. Defaults to `true`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `result` | file (CSV) | CSV encoding of the input Parquet. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000006
  name: base.parquet_to_csv
  inputs:
    - 01890000-0000-0000-0000-000000000000
```
