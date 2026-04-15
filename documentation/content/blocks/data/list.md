+++
title = "data.list"
description = "List objects under a URI prefix."
weight = 5
+++

List objects under a URI prefix, optionally recursively. Returns a JSON array of `{ key, size, last_modified?, etag? }` entries, sorted by key.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `uri` | string | URI prefix to list. |
| `recursive` | boolean | If `true`, walk subprefixes. Defaults to `false`. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `listing` | json | JSON array of listing entries sorted by key. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000104
  name: data.list
  args:
    uri: "s3://my-bucket/prefix/"
    recursive: true
```
