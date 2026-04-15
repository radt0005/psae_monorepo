+++
title = "data.read"
description = "Fetch a single object from any supported storage backend."
weight = 1
+++

Fetch a single object from any supported backend (S3, GCS, Azure Blob, HTTP(S), SFTP, local, WebDAV, Google Drive, OneDrive, Dropbox). The backend is inferred from the URI scheme and hidden from the user.

- **Kind:** `standard`
- **Network:** yes

## Inputs

| Name | Type | Description |
|------|------|-------------|
| `uri` | string | URI of the object to fetch (e.g. `s3://bucket/key`, `https://host/path`, `file:///abs/path`). A bare absolute path is accepted as `file://`. |
| `format` | string | Optional format hint (`GeoTIFF`, `GeoJSON`, `CSV`, `Parquet`, ...). Recorded in the invocation sidecar for downstream reference; not used to validate the bytes. Empty string means "unknown". |

## Outputs

| Name | Type | Description |
|------|------|-------------|
| `file` | file | The fetched object. No format is set on the manifest because it is determined at runtime; wire this output explicitly in downstream pipelines. |

## Example

```yaml
- id: 01890000-0000-0000-0000-000000000100
  name: data.read
  args:
    uri: "s3://my-bucket/input.tif"
    format: "GeoTIFF"
```
