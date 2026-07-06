# User Uploads

This document specifies how users bring their own files -- custom rasters, vector boundaries, tabular data -- into a pipeline, reference them like any other data, share them without re-uploading, and run the same pipeline both locally and in the cloud.

The motivating use case comes from `web_ui.md`: "There should be a way to upload (potentially large) files and use the custom data in pipelines.  There should also be a way to share this data without re-uploading."  Geospatial files are large, so re-transfer and duplication must be avoided.

---

## 1. Headline Decisions

1. **Uploads are a first-party block, not a worker-native source primitive.**  A user-uploaded file is pulled into a pipeline by an ordinary block that the worker executes.  This keeps the "everything is a node" mental model uniform for users and block developers, and reuses the existing execution and type-matching paths rather than adding a new pipeline concept.

2. **A dedicated, deliberately-trusted `cloud` collection.**  The upload blocks live in a `cloud` collection that ships in this repository and is authored by the Spade team.  Trust is granted by **provenance** -- these blocks are first-party -- and that trust is what earns them the one elevated capability they need: receiving a pre-signed URL.

3. **Access via pre-signed URLs minted at dispatch.**  The worker mints a short-lived, single-object pre-signed GET URL for the uploaded file and injects it into the invocation.  No standing storage credential ever lives inside a block, and the blast radius of the capability is a single object.

4. **One block variant per spade type, one UI node.**  Because block output types are static in `block.yaml`, there is a variant per type (`cloud.upload_raster`, `cloud.upload_vector`, ...), all sharing a single implementation.  The web UI presents a **single "Upload" node with a type dropdown** so users never think about which variant they are using.

5. **A user-data catalog.**  An upload is a persistent asset with metadata in PostgreSQL and bytes in Spaces.  Pipelines reference an asset by id; sharing and de-duplication fall out of content addressing.

---

## 2. The `cloud` Collection

The `cloud` collection is the home for **deliberately-trusted, first-party blocks** -- blocks the platform grants capabilities that would be unsafe to grant arbitrary third-party plugins.  The upload blocks are its first members.

- **Provenance is the trust boundary.**  A block earns elevated capability by living in this repository and being authored and reviewed by the Spade team.  A third-party collection cannot join `cloud`, and therefore cannot obtain the capabilities gated to it.
- **The capability gate is an allow-list, not a manifest flag.**  A block does not *declare* "give me a pre-signed URL."  The worker holds an allow-list of trusted upload block names (see §6); only those receive URL injection.  This prevents a third-party block from requesting the capability by imitating a manifest.
- **Distribution is unchanged.**  `cloud` blocks are still built and signed by the registry and verified by workers like any other collection (see `registry.md`).  Screening of first-party source is a policy choice (`registry.md` §4) and may be automatic.

A block that merely uses network and a secret -- for example, one querying a user's database with a `get_secret` connection string -- does **not** belong in `cloud`.  That is an ordinary block that also runs locally.  `cloud` is specifically for blocks that require a platform-granted capability.

---

## 3. The User-Data Catalog

An uploaded file is a first-class, persistent **asset**, not a per-pipeline attachment.

- **Metadata** lives in the managed PostgreSQL cluster (`hosting.md` §6.1): asset id, owner (Better Auth identity), display name, declared spade type, content hash, object key, size, created timestamp.
- **Bytes** live in the `spade-user-data/` Spaces bucket (`hosting.md` §6.2).

Because assets are content-addressed, **sharing requires no re-upload**: sharing an asset is granting another user a reference to the same asset id, and re-using a file across pipelines re-uses the same object key.  The worker-local input cache (`worker.md` Storage Model) de-duplicates fetches by content hash, so a shared reference dataset is fetched once per worker.

Assets are created through the web UI (§8) and, later, a CLI command (`spade data upload`, see §7 and `cli.md`).  Ownership and sharing semantics beyond "owner may read and reference" are out of scope for this document.

---

## 4. Typing

Block output types are declared statically in `block.yaml` (e.g. `outputs: { raster: { type: file, format: GeoTIFF } }`), and the worker's input/output type-matching depends on those static manifests (see `worker.md`).  A single upload block cannot honor this, because an upload may be a raster, a vector, or a table.

Therefore there is **one block variant per spade type**, each with a static output manifest:

| Variant | Output type |
| --- | --- |
| `cloud.upload_raster` | `RasterFile` |
| `cloud.upload_vector` | `VectorFile` |
| `cloud.upload_table` | `TabularFile` |
| `cloud.upload_file` | `File` (untyped passthrough) |

All variants share a single implementation; they differ only in the output declaration in their manifests.

**Type detection.**  The declared type is chosen at upload time and defaults from the file extension, with a user override.  The extension-to-type mapping lives in shared Go (`core`) so the web UI and the CLI resolve it identically.  The upload block passes the file through unchanged -- it does **not** convert formats; the declared type is an assertion about the file the user provided.

---

## 5. The Upload Blocks

Each variant is a thin first-party block.  Manifest sketch (`cloud.upload_raster`):

```yaml
id: cloud.upload_raster
version: 0.1.0
kind: standard
network: true
description: Materialize a user-uploaded raster asset as a pipeline input.
inputs:
  url:
    type: string
    description: Pre-signed GET URL for the asset, injected by the worker.
outputs:
  raster:
    type: file
    format: GeoTIFF
    description: The uploaded raster file.
```

The implementation, shared across variants:

1. Reads `url` from `params.yaml`.
2. Performs an HTTP GET (the block declares `network: true`).
3. Writes the downloaded bytes into `outputs/<name>/`.

The block has **no source block inputs** (`inputs: []` in the pipeline); the asset reference is an argument, and the `url` param is provisioned by the worker at dispatch (§6).

---

## 6. Pre-Signed URL Provisioning

The pipeline stores a **stable asset reference**, never a URL (pre-signed URLs are short-lived and must not be persisted).  The URL is minted at execution time:

1. The pipeline block entry names an allow-listed upload block and carries the asset id in `args` (§7).
2. At dispatch, the **worker** recognizes the block name against its trusted-upload allow-list.  It resolves the asset id to its object key via the user-data catalog, then mints a short-TTL pre-signed GET URL against `spade-user-data/`.  The worker already holds Spaces credentials (`hosting.md` §4.3), so no new credential distribution is required.
3. The worker writes the URL into the invocation's `params.yaml` (as the `url` argument) before starting the sandbox.
4. The block GETs the URL and writes the output.

The **scheduler stays storage-agnostic** -- it does not mint URLs or hold Spaces credentials.  A non-allow-listed block never receives a URL, so a third-party block cannot obtain one by declaring the same argument shape.

Because the URL is provisioned per invocation and expires quickly, at-least-once job redelivery (`worker.md`) is safe: a redelivered job mints a fresh URL.

---

## 7. Pipeline Representation

An upload appears as an ordinary source block.  The pipeline stores the asset id; the worker turns it into a URL at dispatch (§6):

```yaml
blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: cloud.upload_raster
    inputs: []
    args:
      asset_id: 019cf4bc-aaaa-7000-0000-000000000000

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.reproject
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000     # consumes the upload's output
    args:
      target_crs: "EPSG:4326"
```

Downstream blocks reference the upload's invocation id exactly as they reference any block's output, and type-matching resolves via the variant's static output manifest (§4).  No new input-reference form is introduced (contrast with the rejected native-source-node design).

The CLI mirrors this: `spade data upload --type raster boundary.tif` registers the asset and prints its id; a hand-authored pipeline references that id.  `--type` defaults from the extension and overrides the auto-detected type.

---

## 8. Web UI

The user drags a single **"Upload" node** onto the canvas.  It does not require the user to know which block variant backs it.

- The node lets the user upload a new file or pick an existing asset from their catalog (§3), supporting share-without-re-upload.
- A **type dropdown** is auto-populated from the file extension and can be overridden.
- On serialization, the UI emits the matching variant (`cloud.upload_<type>`) with the chosen `asset_id`.  The variant selection is entirely hidden from the user.

Large-file upload mechanics (multipart / resumable transfer, progress) are a web-UI implementation concern and out of scope here.

---

## 9. Local Runs

Locally there is no object storage; the user's file is already on disk.  The upload block is still used, so the pipeline is identical across environments -- only resolution differs.

The CLI resolves the asset id to a local path and injects a `file://` URL into the block's `url` argument instead of a pre-signed HTTP URL.  **The block handles both schemes** (`http(s)://` in the cloud, `file://` locally) through one code path, so `cloud.upload_*` blocks run locally as well as in the cloud.  A `file://` argument's directory is bound into the sandbox by the existing arg-path discovery in the executor (`core/executor.go`), so no network is needed locally.

This mirrors the local/cloud split in secrets management (`secrets.md`): the pipeline references data by a stable id, and where that id resolves depends on where the pipeline runs.

---

## 10. End-to-End Flow (cloud)

1. The user uploads `boundary.tif` in the web UI, tags it `raster` (defaulted from `.tif`), and the platform stores metadata in PostgreSQL and bytes in `spade-user-data/`, returning an asset id.
2. The user drops an Upload node, selects the asset, and connects it to `raster.reproject`.  The UI serializes a `cloud.upload_raster` block with that `asset_id`.
3. On run, the scheduler dispatches the `cloud.upload_raster` invocation.
4. The worker recognizes the allow-listed block, resolves `asset_id` to the object key, mints a short-TTL pre-signed GET URL, and writes it into `params.yaml`.
5. The block GETs the URL and writes `outputs/raster/boundary.tif`.
6. `raster.reproject` consumes it as a normal `RasterFile` input via the worker-local cache.

---

## 11. Out of Scope

- Large-file / resumable upload transport in the web UI and CLI.
- Retention and lifecycle policy for `spade-user-data/` (cleanup of unreferenced assets).
- Cross-user sharing ACLs beyond the owner-read model of §3.
- Format conversion or validation on ingest -- the upload block passes bytes through; the declared type is the user's assertion.
- Additional `cloud` collection blocks beyond uploads.
