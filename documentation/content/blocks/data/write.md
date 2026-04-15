+++
title = "data.write"
description = "Write a single local file to any supported storage backend."
weight = 3
+++

Write a single local file to any supported backend. The destination backend is inferred from the URI scheme. Fails if the destination exists unless `overwrite` is `true`.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `file` | file | Local file to upload. |
| `uri` | string | Destination URI (e.g. `s3://bucket/key`, `file:///abs/path`). |
| `overwrite` | boolean | If `true`, overwrite an existing destination. Defaults to `false`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `receipt` | json | JSON receipt of the write, containing the destination URI, the number of bytes written, and the sha256 of the uploaded content. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000102
  name: data.write
  inputs:
    - 01890000-0000-0000-0000-000000000000
  args:
    uri: "s3://my-bucket/output.parquet"
    overwrite: true
```
