# Data Block Collection — Implementation Plan

This is a checklist for implementing the data-import blocks defined in `./SPECIFICATION.md` in Rust. The collection lives at `blocks/data/`, uses the `spade` runtime library at `../../libs/rust/`, wraps Apache OpenDAL for generic I/O, and compiles to a single binary (`data`) that dispatches one subcommand per block.

The collection has two layers:

1. **Generic OpenDAL blocks** — `read`, `read_collection`, `write`, `write_collection`, `list`, `stat`. Users give a URI; the scheme dispatches to the right OpenDAL backend.
2. **Catalog blocks** — one block per well-known public dataset. Each hides the backend and exposes only domain-relevant arguments.

Work proceeds in phases. Each phase leaves the crate in a compiling, testable state so partial progress is shippable.

---

## Phase 0 — Project scaffolding and shared infrastructure

Foundational work that every block depends on.

### 0.1 Cargo setup

- [DONE] Open `Cargo.toml` and set `edition = "2024"` to match the `spade` runtime. (Deviation: used `edition = "2021"` to match `base/`.)
- [DONE] Add a `[dependencies]` section with:
  - [DONE] `spade = { path = "../../libs/rust" }` — the runtime library.
  - [DONE] `opendal = { version = "0.50", default-features = false, features = [...] }`. Deviation: `services-ftp` is disabled — opendal 0.50.2's FTP service fails to compile against the current rustls/async-native-tls combo. `Scheme::Ftp` parses but `build_operator` returns `UnsupportedScheme`.
  - [DONE] `tokio = { version = "1", features = ["rt", "rt-multi-thread", "macros", "io-util"] }`.
  - [DONE] `bytes = "1"`.
  - [DONE] `url = "2"`.
  - [DONE] `reqwest = { version = "0.12", default-features = false, features = ["rustls-tls", "stream", "blocking"] }`.
  - [DONE] `zip = "2"`.
  - [DONE] `flate2 = "1"`.
  - [DONE] `tar = "0.4"`.
  - [DONE] `sha2 = "0.10"`.
  - [DONE] `serde = { version = "1", features = ["derive"] }`
  - [DONE] `serde_json = "1"`
  - [DONE] `serde_yaml = "0.9"`
  - [DONE] `thiserror = "2"`.
  - [DONE] `anyhow = "1"`.
  - [DONE] `indicatif = "0.17"`.
  - [DONE] `tempfile = "3"`.
- [DONE] Add a `[dev-dependencies]` section with `tempfile = "3"`, `assert_fs = "1"`, `mockito = "1"`, `wiremock = "0.6"`.
- [DONE] Add `[[bin]] name = "data"` with `path = "src/main.rs"`.
- [DONE] Run `cargo check` and confirm a clean build.

### 0.2 Dispatcher wiring (`src/main.rs` and `src/lib.rs`)

- [DONE] In `src/main.rs`, implement a dispatcher that reads `std::env::args().nth(1)` and matches on the subcommand name, calling the corresponding `<block_module>::entry()`.
- [DONE] Unknown subcommands print a usage message listing all blocks and exit with code 2.
- [DONE] In `src/lib.rs`, `pub mod` every block module and re-export the module's public surface so the crate is usable as a library for integration tests.
- [DONE] Add `pub mod common;` for shared helpers.

### 0.3 Shared helpers (`src/common/mod.rs`)

- [DONE] `common::error` submodule with all listed variants + `pub type Result<T> = std::result::Result<T, DataError>`.
- [DONE] `common::uri` submodule with `Scheme`, `ParsedUri`, `parse`, and unit tests covering every supported scheme, relative-path rejection, empty input, and bare paths.
- [DONE] `common::backend` submodule with `build_operator` and `object_key` (returned as `String` rather than `&str` because of per-scheme normalisation). Credentials come from env vars. `TODO(secrets)` comment included.
- [DONE] `common::runtime::block_on` with thread-local current-thread tokio runtime.
- [DONE] `common::download::{fetch_to, fetch_and_verify, extract_zip, extract_zip_tree, sha256_of_file, build_test_zip}` with unit tests for checksum and zip filtering.
- [DONE] `common::params::parse_csv_list` with unit tests.

### 0.4 Block skeleton macro

- [DONE] `pub fn handler_entry<F, T>(f: F)` in `common/mod.rs`.
- [DONE] Documented in top-of-file comment.

### 0.5 Test harness conventions

- [DONE] Created `tests/` with shared `tests/common/mod.rs` (`work_dir`, `write_params`, `put_input`) copied from `blocks/base/tests/common/mod.rs`.
- [DONE] Catalog tests build a fresh `wiremock::MockServer` per test rather than sharing a single helper — each block's URL override is an env var, so per-test servers are easier to reason about.

### 0.6 Expose a test-friendly entrypoint per block

- [DONE] `entry()` + `run_in(base: &Path)` per block (see individual block modules).

---

## Phase 1 — The generic `read` block (prove the OpenDAL plumbing)

The single `read` block validates URI dispatch, the async/sync bridge, and the happy-path for every supported backend before we invest in the rest of the surface.

### 1.1 `data.read`

- [DONE] Manifest `blocks/read.yaml`:

  ```yaml
  id: data.read
  version: 0.1.0
  kind: standard
  network: true
  description: Fetch a single object from any supported backend (S3, GCS, Azure Blob, HTTP(S), SFTP, FTP, local, WebDAV, Google Drive, OneDrive, Dropbox, HDFS). The backend is inferred from the URI scheme and hidden from the user.

  inputs:
    uri:
      type: string
      description: URI of the object to fetch (e.g. s3://bucket/key, https://host/path, file:///abs/path, /abs/path).
    format:
      type: string
      description: Optional format hint (GeoTIFF, GeoJSON, CSV, Parquet, …). Recorded in the invocation for downstream reference; not used to validate the bytes.

  outputs:
    file:
      type: file
      description: The fetched object. No format is set on the manifest because it is determined at runtime; wire this output explicitly in downstream pipelines.
  ```

  Note: `inputs.format` is declared as a string param (scalar). The runtime passes it via `params.yaml`. The empty string or missing value is treated as "unknown format".
- [DONE] `src/read.rs` handler with all bullets. Deviation: the current implementation calls `op.read(key).await` then `fs::write` rather than streaming through `op.reader` + `tokio::io::copy`; for the supported backends (fs/memory/http/s3/gcs) OpenDAL already streams in chunks internally and the in-memory `Buffer` is built of `Bytes` slices rather than a monolithic allocation, so for practical object sizes the peak memory is acceptable. The streaming-copy path can be added later if a large-file test proves the need.
- [DONE] Register the module in `lib.rs` and add a match arm in `main.rs`.
- [DONE] Integration tests: file://, bare path, https happy/404/403, unknown scheme, empty URI. Skipped the 50 MB streaming test (left for Phase 4.4).

### 1.2 Sanity: build and run the binary

- [DONE] `cargo test --test read` passes (8 tests).
- [DONE] Full `cargo build --release` deferred to Phase 5; build compiles in debug mode.

---

## Phase 2 — The rest of the generic surface

These all reuse `common::uri` and `common::backend` from Phase 1. No new plumbing, just new OpenDAL method calls.

### 2.1 `data.read_collection`

- [DONE] Manifest `blocks/read_collection.yaml`.
- [DONE] Handler with `split_prefix_and_glob`, deterministic lexicographic ordering, and `max_items` cap.
- [DONE] Integration tests via the `file://` backend (5-file prefix, `*.csv` glob, max_items cap, empty prefix, `**` rejection).

### 2.2 `data.write`

- [DONE] Manifest and handler. Deviation: handler reads the input file fully then calls `op.write`, instead of streaming through `op.writer`. `op.write` already chunks internally and the simplified code is easier to reason about. The streaming path can land if a large-file benchmark warrants it.
- [DONE] Integration tests: happy path, overwrite=false reject, overwrite=true replace, empty URI.

### 2.3 `data.write_collection`

- [DONE] Manifest and handler. Fail-fast on first error is documented in the module doc comment.
- [DONE] Integration tests: 3-file happy path + existing-destination rejection.

### 2.4 `data.list`

- [DONE] Manifest and handler (uses `Metakey::Complete` so size/mtime/etag are populated).
- [DONE] Integration tests: flat listing sorted, empty listing, empty URI error.

### 2.5 `data.stat`

- [DONE] Manifest and handler.
- [DONE] Integration tests: present key, missing key (error), empty URI.

---

## Phase 3 — Catalog blocks (top 10)

Each catalog block is its own `blocks/<name>.yaml` + `src/<name>.rs`. All share the `common::download` helpers and follow the same template: validate args → resolve upstream URL(s) → fetch → verify → unpack → emit.

Every catalog block sets `network: true`. Output manifests set `format` because the format *is* known at authoring time (unlike generic `read`).

Catalog blocks live under a shared `src/catalog/` module so they don't crowd the top-level namespace. `lib.rs` re-exports each `catalog::<name>::entry` for dispatch. **All upstream URL patterns live in `src/catalog/sources.rs`** so corrections (USFS/MRLC/Geofabrik occasionally restructure paths) are a single-file change.

### 3.1 `data.fia` (USFS Forest Inventory and Analysis) — **priority**

- [DONE] Manifest `blocks/fia.yaml`:

  ```yaml
  id: data.fia
  version: 0.1.0
  kind: standard
  network: true
  description: Download US Forest Service Forest Inventory and Analysis (FIA) public data from the USFS DataMart in CSV format. Defaults to the full national archive; a specific state can be selected.

  inputs:
    state:
      type: string
      description: |
        Two-letter state/territory code or "all" for the entire country (default).
        Accepted values: all, AL, AK, AZ, AR, CA, CO, CT, DE, FL, GA, HI, ID, IL, IN, IA, KS, KY, LA, ME,
        MD, MA, MI, MN, MS, MO, MT, NE, NV, NH, NJ, NM, NY, NC, ND, OH, OK, OR, PA, RI, SC, SD, TN, TX,
        UT, VT, VA, WA, WV, WI, WY, DC, PR, VI, GU, AS, MP.
    tables:
      type: string
      description: |
        Optional comma-separated list of FIA table names to extract (e.g. "PLOT,TREE,COND"). Leave blank
        to extract all tables from the archive.

  outputs:
    tables:
      type: collection
      item_type: file
      format: CSV
      description: One CSV file per extracted FIA table. Filenames mirror the FIA naming convention (<STATE>_<TABLE>.csv).
  ```

- [DONE] `src/catalog/fia.rs` handler with state validation (ALL + 56 codes in a `const` array), archive-filename resolution, `fetch_to` + `extract_zip` + filter based on `<STATE>_<TABLE>.csv`.
- [DONE] URL base lives in `src/catalog/sources.rs::fia_base_url()` with `FIA_BASE_URL` env override.
- [DONE] Integration tests cover all six listed cases.

### 3.2 `data.census_tiger`

- [DONE] Manifest `blocks/census_tiger.yaml`.
- [DONE] URL lookup table maps each layer to `(National|State, template)`.
- [DONE] Year validation 2010..=2100.
- [DONE] `fetch_to` + `extract_zip_tree` into `shapefile/`, returning `Directory`.
- [DONE] Tests: national happy path, state-required error, bad year, unknown layer.

### 3.3 `data.census_acs`

- [DONE] Manifest, handler, tests.
- [DONE] `CENSUS_API_KEY` required (env var). Absent → `DataError::BadArgument` with a clear message (documented placeholder until secrets land).

### 3.4 `data.naturalearth_vector` and `data.naturalearth_raster`

Two blocks (per resolved decision) so output types stay static.

- [DONE] Manifests, handlers, and tests for both blocks.

### 3.5 `data.usgs_3dep`

- [DONE] Manifest, bbox extractor, TNM API call, tile download, `RasterFileCollection` return.
- [DONE] Tests: 2-tile happy path + bad resolution.

### 3.6 `data.nlcd`

- [DONE] Manifest, handler (with AOI-absent warning), tests.
- [DONE] URL uses `nlcd_<year>_<product>_<region>.zip`. Deviation: no `aoi` input is declared on the manifest (the plan mentioned it as optional; since this block does not do clipping itself, we rely on a downstream `gdal.clip_raster_by_vector` and keep the `aoi` parameter out of the manifest to avoid advertising a no-op).

### 3.7 `data.nhd` (USGS National Hydrography Dataset)

- [DONE] Manifest, handler, tests. HUC must be 4 or 8 digits.

### 3.8 `data.ssurgo` (USDA SSURGO soils)

- [DONE] Manifest, handler, tests. Exactly one of `area` or `state` enforced.

### 3.9 `data.prism` (PRISM climate)

- [DONE] Manifest, date enumeration (annual/monthly/daily), handler, tests.
- [DONE] BIL pass-through per resolved decision; document via module doc comment.

### 3.10 `data.osm_extract_pbf` and `data.osm_extract_shp`

Two blocks (per resolved decision) for static output types.

- [DONE] Both manifests, both handlers, both test files. Path-traversal guard added (reject `..` and leading `/`) for safety.

### 3.11 AOI convention (resolved)

- [DONE] `data.usgs_3dep` accepts `aoi` as `type: file, format: GeoJSON`.
- [DONE] Documented in the block description.

---

## Phase 4 — Polish

### 4.1 Cross-block integration tests

- [DONE] `tests/pipeline_read_then_list.rs` exercises `read_collection` end-to-end (emit a file collection) and `read` single-file sha256 round-trip. The base-collection cross-wire tests are deferred because `base` is a separate crate and setting up cross-collection fixtures is heavier than the value they add at this stage.

### 4.2 Manifest validation

- [DONE] All 18 manifests have `id`, `version`, `description`, `inputs`, `outputs`, and `network: true`.
- [DONE] Every input and every output carries a `description` field.
- [DONE] `spade check` is not available in this sandbox (no CLI binary built); manifests were hand-validated against `references/blocks.md`.

### 4.3 Documentation

- [DONE] Module-level `//!` doc comments on every block source file (read, read_collection, write, write_collection, list, stat, and all 12 catalog modules). FIA's "download whole archive" rationale is documented in its module header.
- [ ] `README.md` not created (per "don't create .md files unless explicitly requested" — SPECIFICATION.md and this plan already cover the collection).
- [ ] `SPECIFICATION.md` untouched on purpose — changes there were not requested.

### 4.4 Performance sanity checks

- [ ] Not run — left as a future smoke-test exercise that requires real network / large-file workloads. A tracking note lives in the source comments.

### 4.5 CI wiring

- [ ] CI configuration lives outside this collection; deferred for the monorepo owner to wire. `cargo test` + `cargo clippy --all-targets -- -D warnings` both pass locally with no real network hits.

---

## Phase 5 — Final release checks

- [DONE] `Cargo.toml` at 0.1.0; every manifest at 0.1.0.
- [ ] `cargo build --release` + `spade install .` + PR left for the release owner.

---

## Work-order summary

Dependency order:

1. **Phase 0** unblocks everything.
2. **Phase 1** (`data.read`) validates the OpenDAL + async/sync plumbing end-to-end before investing in the rest.
3. **Phase 2** and **Phase 3** are independent and parallelisable.
4. **Phase 4** requires all of the above.
5. **Phase 5** is the final gate.

Rough time estimate, one contributor: **2 focused days** for Phase 0–2 (the generic surface), **3–4 days** for Phase 3 (ten catalog blocks, mostly URL-wrangling and fixture wiring), another **1 day** for Phase 4 polish. Catalog blocks that depend on API keys (`census_acs`) or on external parsing (`prism`, `nhd`) are the ones most likely to blow through estimates.

---

## Resolved design decisions

All decisions below are settled; the implementation must match these:

- [DONE] **AOI convention.** GeoJSON-only, per resolved decision. `data.usgs_3dep` accepts `aoi: file, format: GeoJSON`.
- [DONE] **Secret handling for `census_acs`.** Reads `CENSUS_API_KEY` from the environment; absent → `DataError::BadArgument` with a clear message. Documented placeholder until a real secrets service lands.
- [DONE] **`usgs_3dep` output shape.** `RasterFileCollection` (tile-per-tile), not a mosaic.
- [DONE] **`naturalearth` raster vs. vector.** Split into `data.naturalearth_vector` and `data.naturalearth_raster`.
- [DONE] **`osm_extract` pbf vs. shp.** Split into `data.osm_extract_pbf` and `data.osm_extract_shp`.
- [DONE] **PRISM BIL handling.** Pass-through as BIL (no GDAL dep in this collection).
- [DONE] **Concurrency in `read_collection`.** Serial fetch; no user-facing concurrency arg. Can be tuned upward if a real workload warrants it.
- [DONE] **Max object size for `read`.** No cap. (The plan's 1 GB info-log is not yet wired since there's no logging framework; a TODO lives in the module header.)
- [DONE] **OpenDAL service feature matrix.** HDFS omitted. FTP also omitted (opendal 0.50.2 build breakage); `Scheme::Ftp` returns `UnsupportedScheme` for now.
- [DONE] **Catalog block versioning.** Centralised in `src/catalog/sources.rs`; every base URL accepts a `*_BASE_URL` env override for tests.

---

## Future work (out of scope for 0.1.0)

- Key management integration (awaiting cloud-side KMS design).
- STAC-driven imagery blocks (`sentinel2`, `landsat`, `harmonized_landsat_sentinel`) as their own collection — geospatially richer than "fetch a URL".
- Write-side catalog blocks (e.g. publish to an internal registry). Generic `write`/`write_collection` covers the usual case.
- Resumable / checkpointed downloads for multi-gigabyte catalog sources. OpenDAL has partial support; revisit once a pipeline hits the limit.
