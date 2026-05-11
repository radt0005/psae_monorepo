# Writing Spade Pipelines

A **pipeline** is a YAML file describing a directed acyclic graph (DAG) of block invocations. It tells the scheduler which blocks to run, in what order, with which arguments, and how their outputs feed into each other's inputs.

Pipelines are usually created in the web UI's flowchart editor, but they can also be authored by hand. The on-disk format is the same in either case.

For hand-authored or LLM-generated pipelines, **short codes** (`@<identifier>`) can be used in place of UUIDv7 invocation IDs. The CLI resolves short codes to UUIDs via a sibling lockfile (`<pipeline-stem>.lock.yaml`); see "Short codes and the lockfile" below.

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
| `id`          | UUIDv7      | no       | Pipeline ID; the CLI generates one at run/submission time if omitted. Hand-authored pipelines typically leave this out. |
| `name`        | string      | yes      | Human-readable pipeline name                         |
| `version`     | string      | yes      | Pipeline version (free-form, conventionally semver)  |
| `description` | string      | no       | Optional description                                  |
| `blocks`      | Block[]     | yes      | Ordered list of block invocations                    |

Block IDs should be UUIDv7 **or** short codes (`@reproject`, `@clip`); see "Short codes and the lockfile". Whichever form is used, block IDs are stable across reruns of the same pipeline — that's how the cache hits.

---

## Block fields

Each entry in `blocks`:

| Field    | Type                 | Required | Description                                          |
| -------- | -------------------- | -------- | ---------------------------------------------------- |
| `id`     | UUIDv7 or short code | yes      | Stable invocation ID; reused on rerun for caching. Short codes (`"@name"`) are resolved by the CLI via the lockfile. |
| `name`   | string               | yes      | The block type ID, e.g. `gdal.rasterize`. Must match an installed block in the registry. |
| `inputs` | InputRef[]           | yes      | Dependencies on other blocks. Use `[]` for source blocks. |
| `args`   | map<string, any>     | yes      | Scalar parameters written to the block's `params.yaml`. Use `{}` if none. |

Pipelines that omit `inputs` or `args` are invalid — write `inputs: []` and `args: {}` for source blocks rather than leaving them out.

---

## Input references

The `inputs` list connects this block to its dependencies. Two structural forms are supported and may be mixed in the same list. Each reference can use either a UUIDv7 or a short code (`@name`) interchangeably.

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

## Short codes and the lockfile

UUIDv7s are painful to author by hand and impossible for an LLM to generate consistently across a multi-block YAML file. Spade pipelines accept **short codes** as an alternative: `@<identifier>` (e.g. `@source`, `@reproject`) used anywhere a block invocation ID is expected.

```yaml
name: s2-reproject-mosaic
version: "1.0"

blocks:
  - id: "@source"
    name: data.sentinel2
    inputs: []
    args: {region: "POLYGON((...))"}

  - id: "@tiles"
    name: base.map_files
    inputs:
      - block: "@source"
        output: tiles
    args: {}

  - id: "@reproject"
    name: raster.reproject
    inputs:
      - "@tiles"
    args: {target_crs: "EPSG:4326"}

  - id: "@mosaic"
    name: base.mosaic
    inputs:
      - "@reproject"
    args: {}
```

Notes:

- **Quote short codes.** `@` is a reserved indicator in YAML 1.2 plain scalars. Quoting (`"@source"`) avoids issues with strict parsers.
- **Identifier grammar.** `@<identifier>` where `<identifier>` matches `[A-Za-z_][A-Za-z0-9_]*`. Numeric labels (`@1`, `@2`) work but named codes are preferred — they survive reordering and read better.
- **Where short codes are allowed.** A block's `id` field, and `inputs` references (bare or explicit). **Not** inside `args` values — those are application data and the CLI must not substitute inside them.
- **Mixing.** UUIDs and short codes can coexist in the same file. Each form resolves independently. There's no need to convert a UUID-form pipeline (e.g. one exported from the web UI) to short-code form before editing.
- **Pipeline-level `id`.** Out of scope for short codes; the CLI generates it at run/submission time.

### Lockfile

The first time `spade check` or `spade run` processes a pipeline with short codes, the CLI mints a UUIDv7 for each unique code and writes the binding to `<pipeline-stem>.lock.yaml`:

```yaml
# pipeline.lock.yaml
pipeline: s2-reproject-mosaic
version: "1.0"
bindings:
  "@source":    019cf4bc-1111-7000-0000-000000000000
  "@tiles":     019cf4bc-2222-7000-0000-000000000000
  "@reproject": 019cf4bc-3333-7000-0000-000000000000
  "@mosaic":    019cf4bc-4444-7000-0000-000000000000
```

The scheduler and worker only ever see the resolved UUID form; everything downstream of the CLI is unchanged.

Lockfile rules:

- **Commit it to version control** alongside the pipeline source. That's how collaborators get the same cache hits.
- **Adding a new short code** appends a binding on the next CLI run.
- **Renaming a short code** mints a fresh UUID — semantically a different block, so the old cache won't hit. That's intentional.
- **Removed short codes** leave orphan bindings; harmless, may be pruned by the CLI.
- **Deleting the lockfile** is the supported reset path. The next CLI run regenerates everything from scratch — useful if the lockfile is suspect or you want a clean rerun.
- **Manual edits are respected** as long as bindings are valid UUIDv7s pointing to short codes that exist in the source.

### Web UI uploads

When a short-code-form pipeline is uploaded to the web UI, the UI resolves short codes by minting fresh UUIDv7s (the same path it uses to assign IDs to blocks added in the flowchart editor). **The local lockfile is not uploaded** — it stays on the developer's machine. Local and cloud runs maintain independent caches, since cache lives next to its compute.

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

1. All block `id` values are unique within the pipeline (UUIDs or short codes).
2. Every invocation ID referenced in `inputs` resolves to a block in the pipeline.
3. Every `name` refers to an installed block in the registry.
4. The dependency graph is acyclic.
5. Input/output types are compatible between connected blocks.
6. Explicit `output` names exist on the dependency block's `block.yaml`.
7. All required `args` for each block are present.
8. Map blocks output `type: expansion`.
9. Reduce blocks accept a `type: collection` input.
10. No nested maps (every `kind: map` is closed by a `kind: reduce` before another map starts).
11. Short codes match `@[A-Za-z_][A-Za-z0-9_]*`; if a lockfile exists, every binding is a valid UUIDv7 and each bound short code appears in the source.

Run it before `spade run`. It will tell you exactly what's wrong, with messages directing you to use explicit references when wiring is ambiguous. For lockfile-related errors, deleting the lockfile is the supported escape hatch — it regenerates from scratch on the next run.

---

## Caching and reruns

When a pipeline is re-run, the same block invocation IDs are reused — they're authored in (UUID form) or pinned via the lockfile (short-code form), not generated at submission time. The cache key for each invocation is derived from:

- The block's `id` and `version` (from the manifest)
- The hashes of all input contents
- The block's `args` (`params.yaml`)
- The runtime environment hash

If nothing has changed, the block is restored from the cache instead of re-executed. A new pipeline ID is generated for each submission so individual runs remain trackable, but the per-block cache hits across runs.

Short-code pipelines preserve this property as long as `<pipeline-stem>.lock.yaml` is committed alongside the source. Deleting the lockfile invalidates all bindings and forces a cold run — useful when you want a clean reset.

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
2. **Pick an ID style.** For hand-authored or LLM-generated pipelines, default to short codes (`"@reproject"`) — easier to write, easier to review. For pipelines edited downstream of the web UI, leave existing UUIDs alone; new blocks can still use short codes alongside them. Reuse existing IDs when editing.
3. **Wire inputs.** Prefer bare references; switch to explicit `block`+`output` when type matching is ambiguous or when clarity is preferred.
4. **Insert `kind: map` and `kind: reduce` blocks** as needed for fan-out/fan-in.
5. **Set `inputs: []` and `args: {}`** for source blocks (don't omit them).
6. **Suggest `spade check pipeline.yaml`.** For short-code pipelines, this is also where the lockfile gets generated.
