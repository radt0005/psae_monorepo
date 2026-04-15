+++
title = "base.map_range"
description = "Fan out over a numeric range."
weight = 9
+++

Fan out over a numeric range `[start, end)` with a configurable `step`. Useful for repeating a simulation N times in parallel without hand-writing the list of seeds.

- **Kind:** `map`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `start` | number | Inclusive start of the range. |
| `end` | number | Exclusive end of the range. |
| `step` | number | Step between items. Defaults to `1`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `manifest` | expansion | Expansion manifest with one item per value in the range. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000009
  name: base.map_range
  args:
    start: 0
    end: 100
    step: 1
```
