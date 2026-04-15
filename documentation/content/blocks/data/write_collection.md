+++
title = "data.write_collection"
description = "Write every file in a local collection to a remote prefix."
weight = 4
+++

Write every file in a local collection to a remote prefix, preserving basenames. Fails fast on the first error (partial writes are left in place; re-run with a cleanup step if needed).

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `files` | collection of file | Local files to upload. |
| `uri` | string | Destination URI prefix (trailing slash recommended). |
| `overwrite` | boolean | If `true`, overwrite existing destinations. Defaults to `false`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `receipts` | json | JSON array of per-file receipts, each with the destination URI, bytes, and sha256 of the uploaded content. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000103
  name: data.write_collection
  inputs:
    - 01890000-0000-0000-0000-000000000000
  args:
    uri: "s3://my-bucket/results/"
    overwrite: true
```
