# User Uploads & the `cloud` Collection — Implementation Plan

This plan implements the uploads design in [`spec/uploads.md`](spec/uploads.md):
users bring their own files into a pipeline through a first-party **upload
block**, reference them like any other data, share without re-uploading, and run
the same pipeline locally and in the cloud.

The upload block lives in a new, deliberately-trusted **`cloud` collection**. It
receives a short-lived **pre-signed URL** minted at dispatch and GETs the bytes;
the sandbox never holds a standing storage credential.

```
upload file ─► user-data catalog (Postgres) + spade-user-data/ (Spaces)
pipeline stores asset ref ─► dispatch mints pre-signed URL ─► cloud.upload_* GETs it ─► outputs/
local run ─► CLI resolves to file:// ─► same block ─► outputs/
```

---

## 0. Decisions (settled in `spec/uploads.md`)

- **First-party block, not a native source node.** Reuses the block execution +
  type-matching path; keeps "everything is a node."
- **`cloud` collection**, repo-only, trusted by provenance. Capability (receiving
  a pre-signed URL) gated by a **worker allow-list of block names**, not a
  manifest flag.
- **Pre-signed URLs minted at dispatch**, single-object, short TTL. No standing
  bucket credential in a block.
- **One variant per spade type** (`cloud.upload_raster/_vector/_table/_file`),
  shared implementation, static output manifests. Web UI = a single "Upload" node
  with a type dropdown that hides the variant.
- **User-data catalog**: metadata in Postgres, bytes in `spade-user-data/`.
- **Local**: same block, `file://` URL; portable pipeline.

---

## 1. Open decisions to settle during implementation

These are choices the spec left to implementation; each has a recommendation.

- **Collection language: Go (decided).** The block is trivial (HTTP GET → write
  file), so language choice is driven by a project goal: **one core collection per
  language, to battle-test each runtime and language library before release.**
  Current coverage is Rust (`base`, `data`), Python (`gdal`, `ml`), and R (`sae`,
  `stats`); Go and TypeScript have none. Building `cloud` in Go closes the Go gap
  and is the **first Go collection**, so validating the Go-collection
  build/publish path through the registry (`registry.md` §5) is explicit,
  intended work here (Phase 6), not a risk to avoid. TypeScript remains the one
  uncovered language after this.

- **Who mints the URL: worker vs scheduler.** `spec/uploads.md` §6 says the
  worker, to keep the scheduler storage-agnostic. But the worker holds no blob
  client today, and minting needs an **ownership check** (which object may this
  pipeline read?). **Recommendation:** keep worker-minting; make object keys
  **owner-namespaced** (`<owner_id>/<content_hash>`) so the worker authorizes by
  checking the key prefix against the pipeline owner — no catalog round-trip, no
  worker→Postgres coupling. This defers cross-user **sharing** (a shared asset
  won't match the prefix); sharing needs catalog-backed grants and is out of scope
  for v1 (§9).

- **Type-detection source of truth.** The extension→type map is canonical in Go
  `core`; the Nuxt web UI can't import Go. **Recommendation:** duplicate a small
  map in the UI (TS) guarded by a shared JSON fixture + test to prevent drift, or
  expose `GET /uploads/detect-type?filename=`. Start with the fixture-guarded
  duplicate; promote to an endpoint only if drift becomes real.

---

## 2. Order of Work

Local-first again: an `upload_file` block works with `file://` before any cloud
infra exists.

| Phase | Deliverable | Value |
| --- | --- | --- |
| 1 | `cloud` collection + upload blocks (URL/file arg) | works locally via `file://` |
| 2 | Type detection (`core` map, CLI `--type`) | correct typing |
| 3 | User-data catalog (store + upload endpoints + transport) | assets persist |
| 4 | Presign in `blob` + dispatch-time minting + ownership authz | **cloud path works** |
| 5 | Web UI Upload node | end-user surface |
| 6 | Wiring, config, docs | production-ready |

---

## 3. Phase 1 — The `cloud` collection and upload blocks

Scaffold `blocks/cloud/` (language per §1) with four block manifests sharing one
implementation. Manifest sketch (`blocks/cloud/blocks/upload_raster.yaml`):
```yaml
id: cloud.upload_raster
version: 0.1.0
kind: standard
network: true
description: Materialize a user-uploaded raster asset as a pipeline input.
inputs:
  url: { type: string, description: Pre-signed GET URL, injected at dispatch. }
outputs:
  raster: { type: file, format: GeoTIFF, description: The uploaded raster file. }
```
Variants: `upload_raster` (RasterFile), `upload_vector` (VectorFile),
`upload_table` (TabularFile), `upload_file` (File). The shared handler:
1. Read `url` from params.
2. If `file://`, copy the local file; if `http(s)://`, GET it.
3. Write bytes to `outputs/<name>/`.

**Test now (no cloud):** a local pipeline with `cloud.upload_file` and
`args: { url: "file:///abs/path/data.bin" }` → downstream consumes the output.
The executor already binds `file://` arg directories into the sandbox
(`core/executor.go` `discoverArgPaths`), so this runs under `spade run` today.

---

## 4. Phase 2 — Type detection

- New `core` function `DetectAssetType(filename string) SpadeType` with the
  canonical extension→type map (`.tif/.tiff→raster`, `.geojson/.shp/.gpkg→vector`,
  `.csv/.parquet→table`, else `file`). Unit-tested.
- CLI `--type` flag overrides the auto-detected default (Phase 5 for the block
  scaffold command; used by `spade data upload` in Phase 3).
- Web UI mirror per §1 (fixture-guarded).

Detection selects **which variant** the UI/CLI emits (`cloud.upload_<type>`) — it
does not change block behavior; the block passes bytes through unchanged.

---

## 5. Phase 3 — User-data catalog

### 5.1 Store
Extend `server/store` (`types.go`, `pgstore.go`, `memstore.go`):
```
user_assets(
  id, owner_id, display_name, spade_type,
  content_hash, object_key,           -- object_key = "<owner_id>/<content_hash>"
  size_bytes, created_at,
  unique(owner_id, content_hash)       -- de-dup: same bytes → same asset
)
```
Methods: `InsertAsset`, `GetAsset(id)`, `ListAssets(owner)`, `DeleteAsset`.
Content addressing gives share-without-re-upload and de-dup (`spec/uploads.md`
§3).

### 5.2 Upload endpoints (server/api)
- `POST /assets` — accept a file (or presigned PUT handshake for large files),
  compute content hash, store bytes to `spade-user-data/<owner>/<hash>`, insert
  catalog row, return asset id + detected type (overridable via `type` field).
- `GET /assets` — list the caller's assets (names/types/ids).
- `DELETE /assets/<id>`.

Auth via `X-Spade-User-Id` for now (matching `server/api/server.go:134`), Better
Auth deferred.

### 5.3 CLI
`spade data upload --type raster boundary.tif` → `POST /assets`, prints the asset
id. `--type` defaults from `core.DetectAssetType`.

---

## 6. Phase 4 — Pre-signed URLs

### 6.1 Add presign to the blob client
Extend `registry/internal/blob` (or lift it to a shared `blob` module) with
`PresignGet(ctx, key, ttl) (string, error)` using the AWS SDK v2
`s3.PresignClient`. Unit-test against the same S3-compatible target the registry
uses.

### 6.2 Give the worker a blob client
The worker gets Spaces creds via cloud-init (`hosting.md` §4.3) but holds no blob
client today. Wire `S3_*` config into the worker and construct a `blob.S3Store`
(read-only + presign). New `runner/` config fields.

### 6.3 Mint at dispatch
In `Worker.Run` (`runner/worker/worker.go:157`), before the executor call:
1. If the invocation's block name is in the **upload allow-list**
   (`cloud.upload_*`), read the asset's `object_key` from the block args.
2. **Authorize:** check `object_key` is prefixed with the pipeline owner's id
   (§1 ownership model). Reject otherwise.
3. `PresignGet(object_key, shortTTL)` → inject the result as the `url` arg before
   `core.Execute` writes `params.yaml`.

The pipeline stores the stable `object_key` (content-addressed), never a URL.
Redelivery is safe — a redelivered job mints a fresh URL (`worker.md`). Never log
the URL (it grants read access until expiry).

*(If §1 resolves to scheduler-minting instead, this logic moves to
`BuildJobForInvocation` at `server/engine/engine.go:472`, which already has
catalog/owner access; the worker then just consumes the injected `url`.)*

### 6.4 Pipeline representation
The web UI/CLI emit `cloud.upload_<type>` with `args: { asset_id, object_key }`.
`object_key` is what the worker presigns; `asset_id` is retained for provenance
and the UI. Downstream blocks reference the invocation id normally; type-matching
uses the variant's static manifest.

---

## 7. Phase 5 — Web UI Upload node

In `web_ui/` (Nuxt):
- An **Upload node** for the flowchart: upload a new file (→ `POST /assets`) or
  pick an existing asset from the catalog (`GET /assets`).
- A **type dropdown**, defaulted from the filename (Phase 2 mirror), overridable.
- On serialize, emit the matching `cloud.upload_<type>` block with `asset_id` +
  `object_key`. The variant is never shown to the user (`spec/uploads.md` §8).
- Large-file transport (multipart/resumable, progress) is a UI concern; a
  presigned-PUT handshake to Spaces keeps large bytes off the API service.

---

## 8. Phase 6 — Wiring, config, docs

- **`docker-compose.yml`**: `spade-user-data` bucket in the S3-compatible dev
  service; `S3_*` env for the worker (Phase 6.2) and server.
- **Allow-list**: define the trusted upload block names in one place in `runner/`
  so the capability gate is auditable.
- **Registry**: validate the `cloud` collection builds and signs through the
  registry (especially if Go — first Go collection, §1). Screening of first-party
  source may be automatic (`registry.md` §4).
- **Docs**: `documentation/content/` — the Upload node and `spade data upload`;
  `cli.md` command surface. `web_ui.md` already points to `uploads.md`.

---

## 9. Cross-cutting & out of scope

**Cross-cutting**
- **Ownership enforcement** on presign is the security boundary (§6.3) — a
  malicious pipeline must not presign another user's object. Owner-prefixed keys
  enforce it without a catalog lookup in v1.
- **Never log pre-signed URLs.**
- **Content addressing** everywhere: object key = `<owner>/<sha256>` so identical
  bytes de-dup and the worker cache reuses across pipelines.

**Out of scope (this plan)**
- Cross-user asset **sharing** (needs catalog-backed grants; breaks the
  owner-prefix authz shortcut — revisit with a catalog check or scheduler-side
  minting).
- Retention/lifecycle cleanup of unreferenced `spade-user-data/` objects.
- Format conversion/validation on ingest (block passes bytes through).
- Additional `cloud` collection blocks beyond uploads.
- Better Auth on the asset endpoints (deferred; `X-Spade-User-Id` stand-in).
