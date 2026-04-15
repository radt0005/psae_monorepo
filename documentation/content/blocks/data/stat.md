+++
title = "data.stat"
description = "Fetch object metadata for a single URI."
weight = 6
+++

Fetch object metadata (size, last-modified, etag, content-type) for a single URI.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `uri` | string | URI of the object to stat. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `metadata` | json | JSON object with fields `key`, `size`, `last_modified`, `etag?`, `content_type?`. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000105
  name: data.stat
  args:
    uri: "s3://my-bucket/input.tif"
```
