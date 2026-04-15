+++
title = "base.reduce_collection"
description = "Gather mapped outputs back into a single collection."
weight = 10
+++

The minimum-viable reducer. Gathers the outputs of N mapped invocations back into a single `collection` output, preserving filenames. Use this when you just need to regroup the mapped results for downstream consumption without any aggregation.

- **Kind:** `reduce`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `items` | collection of file | Outputs of the upstream mapped block. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `result` | collection of file | The same items, regrouped as a collection. |

## Example

```yaml
- id: 01890000-0000-0000-0000-00000000000a
  name: base.reduce_collection
  inputs:
    - 01890000-0000-0000-0000-000000000007
```
