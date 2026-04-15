+++
title = "data.read_collection"
description = "Fetch multiple objects matching a URI prefix or glob."
weight = 2
+++

Fetch multiple objects matching a URI prefix or simple glob (single `*` in the trailing segment). Backend is inferred from the scheme. Use `max_items` to guard against accidentally listing millions of keys.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `uri` | string | URI prefix or glob (e.g. `s3://bucket/prefix/`, `file:///abs/path/*.csv`). `**` is rejected explicitly because recursive listing is easy to misuse. |
| `format` | string | Optional format hint recorded for downstream reference. |
| `max_items` | number | Maximum number of items to fetch. `0` means unlimited. If the backend returns more than this, the block errors out so users must opt in to larger fetches. |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `files` | collection of file | Fetched objects, one file per listed key. Keys with slashes are flattened (slashes become underscores). |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000101
  name: data.read_collection
  args:
    uri: "s3://my-bucket/tiles/*.tif"
    max_items: 100
```
