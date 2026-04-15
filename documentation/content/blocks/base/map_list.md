+++
title = "base.map_list"
description = "Fan out over a literal list of scalar values."
weight = 8
+++

Fan out over a literal list of scalar values (strings, numbers, or booleans) supplied as a parameter. Each value is materialised as a small JSON file recorded in the expansion manifest so downstream blocks have a real file to consume.

- **Kind:** `map`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `values` | string | JSON-encoded array of scalars, e.g. `'["NY","MI","CA"]'` or `'[1,2,3]'`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `manifest` | expansion | Expansion manifest with one item per input value. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000008
  name: base.map_list
  args:
    values: '["NY","MI","CA"]'
```
