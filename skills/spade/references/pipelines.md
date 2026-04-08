# Writing Spade Pipelines

A **pipeline** is a YAML file describing a directed acyclic graph (DAG) of block invocations. It tells the scheduler which blocks to run, in what order, with which arguments, and how their outputs feed into each other's inputs.

Pipelines are usually created in the web UI's flowchart editor, but they can also be authored by hand. The on-disk format is the same in either case.

---

## Top-level structure

```yaml
id: 019cf4bc-0000-7000-0000-000000000000
name: land-cover-classification
version: "1.0"
description: Classifies land cover from Sentinel-2 imagery

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

| Field         | Type        | Required | Description                                          |
| ------------- | ----------- | -------- | ---------------------------------------------------- |
| `id`          | UUIDv7      | yes      | Pipeline ID, generated at submission time            |
| `name`        | string      | yes      | Human-readable pipeline name                         |
| `version`     | string      | yes      | Pipeline version (free-form, conventionally semver)  |
| `description` | string      | no       | Optional description                                  |
| `blocks`      | Block[]     | yes      | Ordered list of block invocations                    |

Block IDs **and** the pipeline ID should be UUIDv7. The block IDs are stable across reruns of the same pipeline — that's how the cache hits.

---

## Block fields

Each entry in `blocks`:

| Field    | Type             | Required | Description                                          |
| -------- | ---------------- | -------- | ---------------------------------------------------- |
| `id`     | UUIDv7           | yes      | Stable invocation ID; reused on rerun for caching    |
| `name`   | string           | yes      | The block type ID, e.g. `gdal.rasterize`. Must match an installed block in the registry. |
| `inputs` | InputRef[]       | yes      | Dependencies on other blocks. Use `[]` for source blocks. |
| `args`   | map<string, any> | yes      | Scalar parameters written to the block's `params.yaml`. Use `{}` if none. |

Pipelines that omit `inputs` or `args` are invalid — write `inputs: []` and `args: {}` for source blocks rather than leaving them out.

---

## Input references

The `inputs` list connects this block to its dependencies. Two forms are supported and may be mixed in the same list.

### Bare reference (just an invocation ID)

```yaml
inputs:
  - 019cf4bc-1111-7000-0000-000000000000
```

The worker resolves which output of the dependency feeds which input of this block by **type matching** against the two `block.yaml` manifests. This works as long as the matching is unambiguous — for example, one GeoTIFF output feeding one GeoTIFF input.

### Explicit reference (`block` + `output`)

```yaml
inputs:
  - block: 019cf4bc-1111-7000-0000-000000000000
    output: clipped_raster
  - block: 019cf4bc-1111-7000-0000-000000000000
    output: metadata
```

This maps a specific named output from the dependency to an input on the current block. Use it when type matching alone is ambiguous.

### Mixed

```yaml
inputs:
  - 019cf4bc-1111-7000-0000-000000000000
  - block: 019cf4bc-2222-7000-0000-000000000000
    output: boundary
```

Explicit references are resolved first; remaining slots are filled by type matching against the bare references.

### When explicit references are required

You **must** use the explicit form when:

- A dependency block has multiple outputs that match the same input type on the current block (e.g. two GeoTIFF outputs and one GeoTIFF input).
- Multiple bare references contribute outputs of the same type that could satisfy the same input.

If the wiring is ambiguous after the resolver runs, the pipeline is **invalid**. There is no positional / ordering fallback. `spade check` and the web UI catch this at authoring time with a clear error.

The explicit form is **recommended** even when not required, especially for pipelines involving map/reduce — it makes the data flow obvious to readers.

### Resolution algorithm (for reasoning about edge cases)

1. Resolve explicit references first; remove the matched inputs and outputs from further consideration.
2. For each remaining bare reference, match the dependency's unmatched outputs to the current block's unmatched inputs by type.
3. If any input is satisfied by more than one candidate, reject the pipeline as ambiguous.
4. If any input has no candidate, reject the pipeline as incomplete.

---

## Map and reduce in pipelines

Map and reduce are not special syntax — they're just blocks with `kind: map` or `kind: reduce` in their manifests. The scheduler reads the kinds and arranges fan-out / fan-in accordingly.

A typical fan-out / fan-in pipeline:

```yaml
blocks:
  # Source: emit a collection of tiles
  - id: 019cf4bc-aaa
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((...))"

  # MAP: enumerate the tiles into individual items
  - id: 019cf4bc-bbb
    name: base.map_files
    inputs:
      - block: 019cf4bc-aaa
        output: tiles
    args: {}

  # Per-tile processing — runs N times in parallel
  - id: 019cf4bc-ccc
    name: raster.ndvi
    inputs:
      - 019cf4bc-bbb
    args: {}

  # Per-tile classification — also N times, with a broadcast model
  - id: 019cf4bc-ddd
    name: raster.classify
    inputs:
      - 019cf4bc-ccc
      - block: 019cf4bc-fff
        output: trained_model      # broadcast: same model for every tile
    args:
      method: random_forest

  # Out-of-context dependency: not part of the map context
  - id: 019cf4bc-fff
    name: ml.load_model
    inputs: []
    args:
      model_path: "models/landcover_rf.pkl"

  # REDUCE: collect all classified tiles into one mosaic
  - id: 019cf4bc-eee
    name: base.mosaic
    inputs:
      - 019cf4bc-ddd
    args: {}
```

Key points:

- **Map context propagates downstream.** Every block between a `kind: map` and the next `kind: reduce` is invoked once per item.
- **Mapped indices are stable through the chain.** `ccc.2` always feeds `ddd.2`, so a single tile can be traced through the whole map context.
- **Non-mapped dependencies are broadcast.** If a block in the map context depends on a block outside the map context, that dependency's output is symlinked into every mapped invocation's `inputs/`. No annotation needed.
- **A reduce block must accept a `collection` input.** It receives all N outputs from the last mapped block as one collection.
- **Nested maps are not supported.** A map context must be closed by a reduce before another map begins. `spade check` enforces this.

---

## Validation rules (`spade check pipeline.yaml`)

`spade check` validates the pipeline against the installed blocks in the registry. It checks:

1. All block `id` values are unique within the pipeline.
2. Every invocation ID referenced in `inputs` exists in the pipeline.
3. Every `name` refers to an installed block in the registry.
4. The dependency graph is acyclic.
5. Input/output types are compatible between connected blocks.
6. Explicit `output` names exist on the dependency block's `block.yaml`.
7. All required `args` for each block are present.
8. Map blocks output `type: expansion`.
9. Reduce blocks accept a `type: collection` input.
10. No nested maps (every `kind: map` is closed by a `kind: reduce` before another map starts).

Run it before `spade run`. It will tell you exactly what's wrong, with messages directing you to use explicit references when wiring is ambiguous.

---

## Caching and reruns

When a pipeline is re-run, the same block invocation IDs are reused — they're authored in, not generated at submission time. The cache key for each invocation is derived from:

- The block's `id` and `version` (from the manifest)
- The hashes of all input contents
- The block's `args` (`params.yaml`)
- The runtime environment hash

If nothing has changed, the block is restored from the cache instead of re-executed. A new pipeline ID is generated for each submission so individual runs remain trackable, but the per-block cache hits across runs.

---

## Worked example: build a pipeline by hand

Goal: download a region of Sentinel-2 imagery, reproject every tile, and mosaic the results.

```yaml
id: 01931e80-0000-7000-8000-000000000000
name: s2-reproject-mosaic
version: "1.0"
description: Download Sentinel-2 tiles, reproject to EPSG:4326, mosaic into a single GeoTIFF

blocks:
  - id: 01931e80-0001-7000-8000-000000000001
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((...))"
      date_range: "2025-01-01/2025-06-01"

  - id: 01931e80-0002-7000-8000-000000000002
    name: base.map_files
    inputs:
      - block: 01931e80-0001-7000-8000-000000000001
        output: tiles
    args: {}

  - id: 01931e80-0003-7000-8000-000000000003
    name: raster.reproject
    inputs:
      - 01931e80-0002-7000-8000-000000000002
    args:
      target_crs: "EPSG:4326"

  - id: 01931e80-0004-7000-8000-000000000004
    name: base.mosaic
    inputs:
      - 01931e80-0003-7000-8000-000000000003
    args: {}
```

Notes:
- The `base.map_files` invocation uses an explicit `output: tiles` because `data.sentinel2` is assumed to publish multiple outputs and we want to be unambiguous.
- `raster.reproject` and `base.mosaic` use bare references because the type matching is one-to-one in each case.
- After saving, run `spade check pipeline.yaml`, then `spade run pipeline.yaml`.

---

## Quick checklist

When asked to write or fix a pipeline:

1. **Identify the blocks** the user needs and look up their inputs/outputs (in the relevant `blocks/*.yaml` files, or from the user's description).
2. **Generate UUIDv7s** for the pipeline and each block. Reuse existing IDs when editing.
3. **Wire inputs.** Prefer bare references; switch to explicit `block`+`output` when type matching is ambiguous or when clarity is preferred.
4. **Insert `kind: map` and `kind: reduce` blocks** as needed for fan-out/fan-in.
5. **Set `inputs: []` and `args: {}`** for source blocks (don't omit them).
6. **Suggest `spade check pipeline.yaml`.**
