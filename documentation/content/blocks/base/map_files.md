+++
title = "base.map_files"
description = "Fan out over every file in a collection for parallel processing."
weight = 7
+++

Enumerate the files of an input collection and emit one expansion item per file. The canonical "for each file in this directory" map block. Downstream blocks receive one invocation per file and run in parallel.

- **Kind:** `map`
- **Network:** no

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `source` | collection of file | Collection of files to fan out over. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `manifest` | expansion | Expansion manifest with one item per input file. Each item's `key` is the file stem and `path` points at the file under `inputs/source/`. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000007
  name: base.map_files
  inputs:
    - 01890000-0000-0000-0000-000000000000
```

See [Map/Reduce](/concepts/map-reduce/) for how downstream blocks consume the expansion.
