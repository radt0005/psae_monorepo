# Pipeline Specification

This document defines the structure of a **pipeline**: the user-authored workflow that connects blocks into a directed acyclic graph (DAG) for execution by the scheduler.

---

## 1. Overview

A pipeline is a declarative description of a data processing workflow.  It lists the blocks to be executed and the dependencies between them.  The scheduler uses this to determine execution order, and the worker uses it to set up each block's invocation directory.

Pipelines are authored in the web UI using a flowchart interface, or by hand in YAML.  The web UI generates the YAML representation before submitting it to the backend.  Hand-authored pipelines may use short codes (see §6) in place of UUIDs to make invocation IDs easier to write and reference.

---

## 2. Pipeline Structure

```yaml
id: 019cf4bc-0000-7000-0000-000000000000
name: land-cover-classification
version: "1.0"
description: Classifies land cover from satellite imagery

blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((...))"
      date_range: "2025-01-01/2025-06-01"

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.reproject
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
    args:
      target_crs: "EPSG:4326"

  - id: 019cf4bc-3333-7000-0000-000000000000
    name: raster.clip
    inputs:
      - block: 019cf4bc-2222-7000-0000-000000000000
        output: clipped_raster
    args:
      boundary: "area_of_interest.geojson"
```

The same pipeline in short-code form is shown in §6.2.

---

## 3. Pipeline Fields

| Field         | Type     | Required | Description                                          |
| ------------- | -------- | -------- | ---------------------------------------------------- |
| `id`          | UUIDv7   | Yes      | Unique pipeline identifier, generated at submission   |
| `name`        | string   | Yes      | Human-readable pipeline name                         |
| `version`     | string   | Yes      | Pipeline version                                     |
| `description` | string   | No       | Optional description of the pipeline's purpose       |
| `blocks`      | Block[]  | Yes      | Ordered list of blocks in the pipeline               |

---

## 4. Block Fields (within a pipeline)

Each entry in `blocks` represents a single invocation of a block type.

| Field    | Type               | Required | Description                                           |
| -------- | ------------------ | -------- | ----------------------------------------------------- |
| `id`     | UUIDv7 or short code | Yes    | Invocation ID.  In the resolved form, a UUIDv7 generated on the client when the block is added to the pipeline (web UI) or assigned via lockfile resolution (CLI; see §6).  Stable across reruns of the same pipeline, which enables caching. |
| `name`   | string             | Yes      | Block type identifier (e.g. `raster.reproject`). Used to look up the block's `block.yaml` manifest and executable. |
| `inputs` | InputRef[]         | Yes      | Dependencies on other blocks in this pipeline. Empty list (`[]`) for source blocks with no dependencies. |
| `args`   | map<string, any>   | Yes      | Key-value parameters written to the block's `params.yaml` at invocation time. Empty object (`{}`) if no parameters. |
| `secrets`| map<string, string>| No       | Binds the logical secret names the block requests (via `get_secret`) to the user's stored secret names. Values are secret **names**, never secret values. Absent means the block declares no secrets. See `secrets.md`. |

---

## 5. Input References

References to other blocks can be expressed using either a UUIDv7 or a short code (see §6).  The two forms are interchangeable wherever an invocation ID is expected, and may be mixed within a single pipeline file.

The `inputs` field supports two structural forms, which can be mixed freely within a single block:

### 5.1 Simple Reference (bare invocation ID)

```yaml
inputs:
  - 019cf4bc-1111-7000-0000-000000000000
  - 019cf4bc-2222-7000-0000-000000000000
```

When a bare invocation ID is used, the worker resolves which outputs from the dependency map to which inputs on the current block using **type matching**: the output types declared in the dependency's `block.yaml` are matched to the input types declared in the current block's `block.yaml`.

This works when the type signatures are unambiguous (e.g. one GeoTIFF output feeding one GeoTIFF input).  **If type matching is ambiguous** -- for example, a dependency has two GeoTIFF outputs and the current block has two GeoTIFF inputs -- **the pipeline is invalid and must use explicit references instead.**  There is no ordering-based fallback.  `spade check` and the web UI catch this at authoring time with a clear error message.

### 5.2 Explicit Reference (named output mapping)

```yaml
inputs:
  - block: 019cf4bc-1111-7000-0000-000000000000
    output: clipped_raster
  - block: 019cf4bc-1111-7000-0000-000000000000
    output: metadata
```

When a block has multiple outputs of the same type, or when clarity is preferred, the explicit form maps a specific **named output** (as declared in the dependency block's `block.yaml` `outputs` section) to an input on the current block.

The explicit form is **required** when:
- A dependency block has multiple outputs that match the same input type (ambiguous type matching)
- Multiple bare references to different dependency blocks produce outputs of the same type that could satisfy the same input

The explicit form is **optional but recommended** when:
- The pipeline author wants to be precise about data flow
- The pipeline involves map/reduce operations (see `scheduler.md`)

### 5.3 Mixed References

Both forms can appear in the same `inputs` list:

```yaml
inputs:
  - 019cf4bc-1111-7000-0000-000000000000
  - block: 019cf4bc-2222-7000-0000-000000000000
    output: boundary
```

Explicit references are resolved first.  Remaining inputs are resolved using type matching against the bare references.  If the remaining matches are still ambiguous after explicit references are removed, the pipeline is invalid.

### 5.4 Resolution Algorithm

The full input resolution algorithm is:

1. **Resolve explicit references**: Match each `block` + `output` reference to the named output of the dependency.  Remove the matched inputs and outputs from further consideration.
2. **Resolve bare references by type**: For each remaining bare reference, match the dependency's unmatched outputs to the current block's unmatched inputs by type.
3. **Check for ambiguity**: If any step produces multiple possible matches for the same input (e.g. two unmatched GeoTIFF outputs for one GeoTIFF input), **reject the pipeline** with an error directing the user to use explicit references.
4. **Check for completeness**: If any input has no match, reject the pipeline with an error.

---

## 6. Short Codes and Lockfile

For hand-authored pipelines -- including those written by a researcher experimenting locally and those generated by an LLM -- requiring UUIDv7 invocation IDs is a usability cost.  UUIDs are difficult to type correctly, easy to miscopy when used as a reference, and impossible for a model to generate consistently across a multi-block file in a single pass.

The pipeline format supports an alternative authoring form: **short codes**.  A short code is the `@` character followed by an identifier, used in place of a UUID anywhere a block invocation ID is expected.  Short codes are resolved to concrete UUIDv7s by the CLI, and the binding is persisted to a sibling **lockfile** so that the cache property described in §11 is preserved across reruns.

The scheduler, worker, web UI, and registry never see short codes.  Resolution happens entirely in the CLI before submission; downstream components consume only the UUID-form pipeline.

### 6.1 Short Code Form

A short code is `@<identifier>` where `<identifier>` matches `[A-Za-z_][A-Za-z0-9_]*`.  Numeric-only identifiers (`@1`, `@2`) are accepted as a special case for very small pipelines, but named short codes (`@reproject`, `@clip`) are preferred -- they survive reordering, are diff-friendly, and read better in review and in LLM-generated output.

Short codes may appear in two places:

- As a block's `id` field
- In any `inputs` reference, either bare (`@reproject`) or explicit (`block: @reproject`)

The pipeline's top-level `id` field is generated at submission time (see §10) and is not part of the short-code system.

Short codes may **not** appear inside `args` values.  Those are passed through to `params.yaml` as application data, not pipeline structure, and the CLI must not substitute inside them.

Two blocks in the same pipeline must not share a short code.  A short code and a UUID may coexist in the same file (see §6.5).

The `@` character is a reserved indicator in YAML 1.2 plain scalars, so short codes **should be quoted** when used as a scalar value.  The CLI's parser accepts both quoted and unquoted forms in practice, but quoting avoids ambiguity with strict YAML parsers downstream.

### 6.2 Example

The same pipeline shown in §2, in short-code form:

```yaml
name: land-cover-classification
version: "1.0"
description: Classifies land cover from satellite imagery

blocks:
  - id: "@source"
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((...))"
      date_range: "2025-01-01/2025-06-01"

  - id: "@reproject"
    name: raster.reproject
    inputs:
      - "@source"
    args:
      target_crs: "EPSG:4326"

  - id: "@clip"
    name: raster.clip
    inputs:
      - block: "@reproject"
        output: clipped_raster
    args:
      boundary: "area_of_interest.geojson"
```

Note that the pipeline-level `id` is omitted; the CLI generates one at run/submission time.

### 6.3 Lockfile

The first time `spade check` or `spade run` processes a pipeline containing short codes, the CLI generates a UUIDv7 for each unique short code and writes the binding to a sibling file `<pipeline-stem>.lock.yaml` (for `pipeline.yaml`, this is `pipeline.lock.yaml`).

```yaml
# pipeline.lock.yaml
pipeline: land-cover-classification
version: "1.0"
bindings:
  "@source":    019cf4bc-1111-7000-0000-000000000000
  "@reproject": 019cf4bc-2222-7000-0000-000000000000
  "@clip":      019cf4bc-3333-7000-0000-000000000000
```

On subsequent invocations:

1. The CLI reads the lockfile, if present.
2. For each short code in the source, it uses the bound UUID from the lockfile.
3. For each short code not yet in the lockfile, it mints a fresh UUIDv7 and appends a new binding.
4. The lockfile is rewritten if any bindings were added.

The resolved (UUID-only) pipeline is the form the scheduler and worker actually consume.

### 6.4 Lockfile Semantics

The lockfile is treated as **authoritative but rebuildable**:

- **Stable bindings enable caching.**  Because the lockfile preserves short code → UUID bindings across reruns, the cache property described in §11 is preserved exactly as it is for fully UUID-authored pipelines.
- **Adding a short code adds a binding.**  Editing the source to introduce `@filter` mints a fresh UUID for it on the next CLI invocation; existing bindings are untouched.
- **Removing a short code leaves an orphan binding.**  Orphans are harmless and may be pruned by the CLI on rewrite; the CLI must not error on them.
- **Renaming a short code mints a fresh binding.**  Changing `@reproject` to `@reproject_satellite` means the new short code has no binding yet and gets a fresh UUID.  Caches from the old name will not hit.  This matches the user's intent: a renamed block is a different block.
- **Deleting the lockfile regenerates everything.**  Removing `pipeline.lock.yaml` is an explicit reset; the next CLI invocation regenerates all bindings, invalidating any caches.  This is the supported escape hatch when the lockfile is suspect or the user wants a clean run from scratch.
- **Manual edits are respected.**  If the user edits the lockfile by hand, the CLI uses whatever UUIDs are present so long as they are valid UUIDv7s and each bound short code appears in the source.  This supports unusual workflows like manually migrating bindings or reusing UUIDs across pipeline files to share cache.

The lockfile is intended to be committed to version control alongside the pipeline source, the same way `Cargo.lock` is committed for binary crates.  This lets collaborators reproduce cache hits.

### 6.5 Mixed-Format Pipelines

A single pipeline file may contain both UUIDs and short codes.  Each form resolves independently:

- UUIDs pass through unchanged.
- Short codes are resolved via the lockfile as described in §6.3.

There is no requirement to convert a UUID-form pipeline -- for example, one exported from the web UI -- into short-code form to edit it locally.  New blocks added to such a pipeline can use short codes; existing blocks can be referenced by their UUID via copy-paste.

### 6.6 Validation

In addition to the rules in §8, `spade check` must verify:

1. Every short code referenced in `inputs` is defined as the `id` of some block in the pipeline.
2. No two blocks share the same short code.
3. Short code identifiers conform to `[A-Za-z_][A-Za-z0-9_]*` after the leading `@`.
4. If a lockfile is present, every binding's UUID is a valid UUIDv7 and every bound short code appears in the source (orphans are tolerated; see §6.4).

Failures in lockfile entries should produce errors that point the user to the option of deleting the lockfile to regenerate it.

### 6.7 Web UI Compatibility

The web UI continues to operate exclusively on resolved (UUID-form) pipelines.  When a user uploads a pipeline that contains short codes:

1. The web UI resolves them by minting fresh UUIDv7s -- the same path it uses to assign UUIDs to blocks added in the flowchart editor.
2. The resolved pipeline is stored in the database.
3. The local lockfile is **not** transferred to the server.  It remains on the developer's machine.

Local runs and cloud runs therefore maintain independent caches, since cache lookups are keyed by invocation UUID and the cloud's UUIDs are minted fresh at upload time.  This split is deliberate: the cache lives next to its compute.

---

## 7. Dependency Graph

The blocks in a pipeline form a DAG.  The scheduler derives the execution order from the dependency relationships expressed in the `inputs` fields.

- Blocks with `inputs: []` are **source blocks** and can be executed immediately.
- A block is ready to execute when **all** of its dependencies have completed successfully.
- Blocks with no dependency relationship between them **may be executed in parallel**.
- Cycles are invalid and should be rejected at validation time (`spade check`).

---

## 8. Validation

The CLI command `spade check` validates a pipeline file.  It should verify:

1. All block `id` values are unique within the pipeline (whether expressed as UUIDs or short codes)
2. All invocation IDs referenced in `inputs` resolve to a block in the pipeline
3. All `name` values refer to known, installed block types
4. The dependency graph is acyclic
5. Input/output type compatibility between connected blocks
6. Named output references (explicit form) match outputs declared in the dependency block's `block.yaml`
7. All required `args` for each block are present
8. For pipelines containing short codes, the rules in §6.6 are additionally satisfied

---

## 9. Map and Reduce

Pipelines express parallel "for each" operations using **map blocks** and **reduce blocks**.  These are regular blocks with `kind: map` or `kind: reduce` in their `block.yaml` manifests.  No special annotation is needed on the pipeline connections -- the scheduler reads the `kind` field to determine behavior.

### 9.1 Example

```yaml
blocks:
  # Download satellite imagery (outputs a collection of tiles)
  - id: 019cf4bc-aaa
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((...))"

  # MAP: enumerate the tiles for fan-out
  - id: 019cf4bc-bbb
    name: core.map.files
    inputs:
      - block: 019cf4bc-aaa
        output: tiles

  # Process each tile (runs N times, one per tile)
  - id: 019cf4bc-ccc
    name: raster.ndvi
    inputs:
      - 019cf4bc-bbb

  # Classify each tile (runs N times, inherits map context)
  - id: 019cf4bc-ddd
    name: raster.classify
    inputs:
      - 019cf4bc-ccc
      - block: 019cf4bc-fff
        output: trained_model    # broadcast: same model for every tile
    args:
      method: random_forest

  # A pre-trained model (not in the map context)
  - id: 019cf4bc-fff
    name: ml.load_model
    inputs: []
    args:
      model_path: "models/landcover_rf.pkl"

  # REDUCE: mosaic all classified tiles into one raster
  - id: 019cf4bc-eee
    name: core.reduce.mosaic
    inputs:
      - 019cf4bc-ddd
```

### 9.2 How It Works

1. The scheduler detects `core.map.files` is a `kind: map` block and dispatches it to a worker
2. The worker executes the map block, which enumerates the tiles and writes an expansion manifest
3. The worker reports the item list to the scheduler
4. The scheduler creates N invocations of `raster.ndvi`: `019cf4bc-ccc.0`, `019cf4bc-ccc.1`, ...
5. Downstream blocks in the map context (`raster.classify`) also get N invocations: `019cf4bc-ddd.0`, `019cf4bc-ddd.1`, ...
6. The non-mapped dependency (`ml.load_model`) is broadcast to all invocations
7. When all mapped invocations complete, the reduce block (`core.reduce.mosaic`) runs once, receiving all outputs as a collection

See `scheduler.md` for the full specification of map/reduce mechanics, including invocation ID schemes, caching, and map context propagation.

### 9.3 Validation

`spade check` should additionally verify for pipelines with map/reduce:

1. Every `kind: map` block's context is closed by a `kind: reduce` block in the dependency graph
2. Contexts are **well-nested**: a block may not combine the outputs of two map contexts unless at least one has been closed by its reduce first (no "merging" of sibling fan-outs), and a reduce closes exactly the innermost open context it consumes from
3. Nesting depth does not exceed the supported maximum (currently 4); invocation counts multiply per level, so this bounds worst-case fan-out
4. Map blocks output the `expansion` type
5. Reduce blocks accept a `collection` input

Nested map/reduce (a mapped block whose output is mapped again before the outer reduce) is supported; see `scheduler.md` §Nested Maps for the execution model.

---

## 10. Lifecycle

1. **Authoring**: The user creates a pipeline in the web UI (flowchart editor) or writes YAML by hand.  Hand-authored pipelines may use short codes (see §6) in place of UUIDs.
2. **ID Assignment**: Block invocation IDs (UUIDv7) are generated when blocks are added to the pipeline (web UI) or when the CLI resolves short codes via the lockfile (see §6).  The pipeline ID is generated at submission time.
3. **Submission**: The pipeline YAML is sent to the backend (PostgreSQL), which stores it and makes it available for the scheduler.  If the pipeline contains short codes at submission time (e.g. uploaded directly to the web UI), they are resolved at upload (see §6.7).
4. **Scheduling**: The scheduler picks up the pipeline, builds the dependency graph, and begins scheduling blocks for execution on workers.
5. **Execution**: Workers execute blocks and report results back to the scheduler.
6. **Completion**: The pipeline is marked as complete (success or failure) once all blocks have finished or a block has failed.

---

## 11. Re-running Pipelines

When a pipeline is re-run, the same block invocation IDs are reused (since they were generated at authoring time, not at submission time).  This enables the caching system to detect that a block with the same ID, inputs, and parameters has already been executed, and skip re-execution if the cached outputs are still valid.

A new pipeline ID is generated for each submission, so each run is independently trackable even when the block IDs are identical.

For pipelines authored in short-code form (see §6), the lockfile preserves the short-code-to-UUID bindings across reruns and across collaborators who share the lockfile alongside the pipeline source.  The cache property described above applies identically whether the pipeline was authored with UUIDs or with short codes.