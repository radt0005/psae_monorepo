# Pipeline Specification

This document defines the structure of a **pipeline**: the user-authored workflow that connects blocks into a directed acyclic graph (DAG) for execution by the scheduler.

---

## 1. Overview

A pipeline is a declarative description of a data processing workflow.  It lists the blocks to be executed and the dependencies between them.  The scheduler uses this to determine execution order, and the worker uses it to set up each block's invocation directory.

Pipelines are authored in the web UI using a flowchart interface, or by hand in YAML.  The web UI generates the YAML representation before submitting it to the backend.

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
| `id`     | UUIDv7             | Yes      | Invocation ID, generated on the client at creation time. Stable across reruns of the same pipeline, which enables caching. |
| `name`   | string             | Yes      | Block type identifier (e.g. `raster.reproject`). Used to look up the block's `block.yaml` manifest and executable. |
| `inputs` | InputRef[]         | Yes      | Dependencies on other blocks in this pipeline. Empty list (`[]`) for source blocks with no dependencies. |
| `args`   | map<string, any>   | Yes      | Key-value parameters written to the block's `params.yaml` at invocation time. Empty object (`{}`) if no parameters. |

---

## 5. Input References

The `inputs` field supports two forms, which can be mixed freely within a single block:

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

## 6. Dependency Graph

The blocks in a pipeline form a DAG.  The scheduler derives the execution order from the dependency relationships expressed in the `inputs` fields.

- Blocks with `inputs: []` are **source blocks** and can be executed immediately.
- A block is ready to execute when **all** of its dependencies have completed successfully.
- Blocks with no dependency relationship between them **may be executed in parallel**.
- Cycles are invalid and should be rejected at validation time (`spade check`).

---

## 7. Validation

The CLI command `spade check` validates a pipeline file.  It should verify:

1. All block `id` values are unique within the pipeline
2. All invocation IDs referenced in `inputs` exist in the pipeline's block list
3. All `name` values refer to known, installed block types
4. The dependency graph is acyclic
5. Input/output type compatibility between connected blocks
6. Named output references (explicit form) match outputs declared in the dependency block's `block.yaml`
7. All required `args` for each block are present

---

## 8. Map and Reduce

Pipelines express parallel "for each" operations using **map blocks** and **reduce blocks**.  These are regular blocks with `kind: map` or `kind: reduce` in their `block.yaml` manifests.  No special annotation is needed on the pipeline connections -- the scheduler reads the `kind` field to determine behavior.

### 8.1 Example

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

### 8.2 How It Works

1. The scheduler detects `core.map.files` is a `kind: map` block and dispatches it to a worker
2. The worker executes the map block, which enumerates the tiles and writes an expansion manifest
3. The worker reports the item list to the scheduler
4. The scheduler creates N invocations of `raster.ndvi`: `019cf4bc-ccc.0`, `019cf4bc-ccc.1`, ...
5. Downstream blocks in the map context (`raster.classify`) also get N invocations: `019cf4bc-ddd.0`, `019cf4bc-ddd.1`, ...
6. The non-mapped dependency (`ml.load_model`) is broadcast to all invocations
7. When all mapped invocations complete, the reduce block (`core.reduce.mosaic`) runs once, receiving all outputs as a collection

See `scheduler.md` for the full specification of map/reduce mechanics, including invocation ID schemes, caching, and map context propagation.

### 8.3 Validation

`spade check` should additionally verify for pipelines with map/reduce:

1. Every `kind: map` block is eventually followed by a `kind: reduce` block in the dependency graph
2. No nested maps (a map context must be closed by a reduce before another map begins)
3. Map blocks output the `expansion` type
4. Reduce blocks accept a `collection` input

---

## 9. Lifecycle

1. **Authoring**: The user creates a pipeline in the web UI (flowchart editor) or writes YAML by hand
2. **ID Assignment**: Block invocation IDs (UUIDv7) are generated on the client when blocks are added to the pipeline.  The pipeline ID is generated at submission time.
3. **Submission**: The pipeline YAML is sent to the backend (PocketBase), which stores it and makes it available for the scheduler
4. **Scheduling**: The scheduler picks up the pipeline, builds the dependency graph, and begins scheduling blocks for execution on workers
5. **Execution**: Workers execute blocks and report results back to the scheduler
6. **Completion**: The pipeline is marked as complete (success or failure) once all blocks have finished or a block has failed

---

## 10. Re-running Pipelines

When a pipeline is re-run, the same block invocation IDs are reused (since they were generated at authoring time, not at submission time).  This enables the caching system to detect that a block with the same ID, inputs, and parameters has already been executed, and skip re-execution if the cached outputs are still valid.

A new pipeline ID is generated for each submission, so each run is independently trackable even when the block IDs are identical.
