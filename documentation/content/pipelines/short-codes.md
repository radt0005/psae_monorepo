+++
title = "Short Codes and Hand-Authoring"
description = "Use @-prefixed short codes instead of UUIDs when writing pipelines by hand."
weight = 3
+++

When you author a pipeline by hand, typing UUIDv7 invocation IDs and copying them into `inputs` references is tedious and error-prone. Spade pipelines accept an alternative authoring form: **short codes**. A short code is `@<identifier>` (for example, `@source`, `@reproject`, `@clip`) used anywhere a block invocation ID is expected.

Short codes are resolved to UUIDv7s by the CLI on `spade check` or `spade run`. The bindings are persisted to a sibling `<pipeline-stem>.lock.yaml` lockfile so the same short code always resolves to the same UUID across reruns -- this is how caching keeps working when you author with short codes.

The scheduler, worker, and web UI never see short codes. Resolution happens entirely in the CLI before the pipeline reaches any of those systems.

## When to use short codes

Use short codes when you are:

- **Writing a pipeline by hand.** Named labels like `@reproject_satellite_data` are far easier to read and reference than `019cf4bc-2222-7000-0000-000000000002`.
- **Generating a pipeline with an LLM.** Language models are essentially incapable of generating valid, internally-consistent UUIDv7s across a multi-block file. Short codes let them produce correct pipelines on the first try.
- **Editing a pipeline collaboratively.** Diffs are much cleaner when blocks are referenced by name -- renaming or reordering doesn't churn UUIDs across the file.

Use UUIDs (or leave them as-is) when:

- **The pipeline came from the web UI flowchart editor.** The web UI emits resolved UUID-form pipelines; there is no need to convert them to short-code form to edit.
- **You need a specific UUID** for cache-sharing or external traceability reasons.

The two forms can be mixed freely in the same file -- see [Mixed-format pipelines](#mixed-format-pipelines) below.

## Short code syntax

A short code is the `@` character followed by an identifier. Identifiers match the pattern `[A-Za-z_][A-Za-z0-9_]*`:

| Valid               | Invalid                            |
|---------------------|------------------------------------|
| `@source`           | `@1bad` (starts with a digit)      |
| `@reproject`        | `@my-block` (hyphen not allowed)   |
| `@map_files_1`      | `@my.block` (dot not allowed)      |
| `@_internal`        | `@` (no identifier)                |
| `@A_B_2`            | `@my block` (space not allowed)    |

Quote short codes in YAML scalar context. The `@` character is a reserved indicator in YAML 1.2 and some strict parsers will reject an unquoted `@source`. Spade's parser accepts both, but quoting is portable:

```yaml
- id: "@source"             # always works
- id: @source               # works with most parsers; not portable
```

## Where short codes are allowed

Short codes may appear in **two places only**:

1. As a block's `id` field.
2. In any `inputs` reference -- either bare (`"@reproject"`) or explicit (`block: "@reproject"`).

Short codes are **not** substituted anywhere else. In particular:

- The pipeline-level `id` field rejects short codes. Omit it (the CLI generates one at run time) or use a concrete UUID.
- Values inside `args` are passed through verbatim. If your block takes a string like `tag: "@latest"` as a parameter, that literal `@latest` is preserved.
- The `output:` field of an explicit reference is an output name, not an invocation ID, and is never touched.

## Hand-authoring example

Here is a pipeline written entirely with short codes:

```yaml
# pipeline.yaml -- a hand-authored pipeline using short codes
name: satellite-reproject
version: "1.0"
description: Download Sentinel-2 imagery and reproject it to EPSG:4326

blocks:
  - id: "@source"
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"
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

Notice that the pipeline-level `id` is omitted -- the CLI will generate one at run time. This is the recommended pattern for hand-authored pipelines.

## The lockfile

The first time you run `spade check` or `spade run` on a pipeline that contains short codes, the CLI generates a UUIDv7 for each unique short code and writes the bindings to a sibling file named `<pipeline-stem>.lock.yaml`. For `pipeline.yaml`, this is `pipeline.lock.yaml`:

```yaml
# pipeline.lock.yaml
pipeline: satellite-reproject
version: "1.0"
bindings:
  "@clip":      019cf4bc-3333-7000-0000-000000000003
  "@reproject": 019cf4bc-2222-7000-0000-000000000002
  "@source":    019cf4bc-1111-7000-0000-000000000001
```

On subsequent runs, the CLI consults the lockfile and reuses the same UUIDs -- this is how the cache continues to hit. New short codes added to the pipeline get fresh bindings appended; the file is rewritten only when bindings change.

{% note() %}
The lockfile is the **source of stability** for short-code pipelines. Commit it to version control alongside the pipeline source, the same way you would commit a `Cargo.lock` or `package-lock.json` file. This is how collaborators reproduce your cache hits.
{% end %}

### Lockfile rules

The lockfile is treated as **authoritative but rebuildable**:

| Action on the source | Effect on the lockfile |
|---------------------|------------------------|
| Add a new short code | A fresh binding is appended on the next CLI run. |
| Reuse an existing short code | The existing binding is reused; cache continues to hit. |
| Rename a short code (e.g. `@reproject` → `@reproject_v2`) | The new name gets a fresh UUID. The old binding becomes an orphan and is preserved (harmless). |
| Remove a short code from the source | Its binding becomes an orphan. Validation does not fail; the binding may be pruned on rewrite. |
| Delete the lockfile entirely | The next CLI run regenerates all bindings from scratch. This is the **supported reset path** -- use it whenever the lockfile is suspect or you want a guaranteed clean rerun. |
| Edit the lockfile by hand | Respected as long as bindings are valid UUIDv7s pointing to short codes that exist in the source. Useful for unusual workflows like reusing UUIDs across files to share caches. |

### Caching and short codes

The pipeline cache keys block executions on `(block id, block version, input hashes, args, runtime environment)`. Because the lockfile pins each short code to a stable UUID across reruns, the cache property described in the [pipeline format reference](/pipelines/format/) applies identically whether you authored with UUIDs or with short codes:

```bash
# First run: lockfile is created; all blocks execute.
spade run pipeline.yaml

# Second run: lockfile reused; blocks restored from cache.
spade run pipeline.yaml
#  [1/3] data.sentinel2 (cached)
#  [2/3] raster.reproject (cached)
#  [3/3] raster.clip (cached)

# Delete the lockfile: fresh UUIDs, cold cache.
rm pipeline.lock.yaml
spade run pipeline.yaml
#  [1/3] data.sentinel2 running...
```

## Mixed-format pipelines

Short codes and concrete UUIDs can coexist freely. There is no requirement to convert a UUID-form pipeline (for example, one exported from the web UI) into short-code form to edit it locally. Add new blocks using short codes; reference existing UUID-form blocks by copy-pasting their UUID:

```yaml
blocks:
  # Existing UUID-form block, e.g. exported from the web UI
  - id: 019cf4bc-1111-7000-0000-000000000001
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((...))"

  # New hand-authored block using a short code, referencing
  # the existing block by its UUID
  - id: "@filter"
    name: raster.filter
    inputs:
      - 019cf4bc-1111-7000-0000-000000000001
    args:
      threshold: 0.5
```

When `spade check` or `spade run` processes this file, the UUID passes through unchanged and only `@filter` is resolved against the lockfile.

## Working with the web UI

The web UI continues to operate exclusively on resolved (UUID-form) pipelines. When you upload a short-code-form pipeline through the web UI, the server resolves short codes by minting fresh UUIDv7s -- the same path it uses when assigning IDs to blocks added in the flowchart editor.

The local lockfile is **not** uploaded. It stays on the developer's machine. This means local runs (via `spade run`) and cloud runs (via the web UI) maintain independent caches, since each environment has its own UUIDs. This split is deliberate -- the cache lives next to its compute.

If you need cache parity between local and cloud, you can manually copy the lockfile's UUIDs into the YAML before uploading, but in practice this is rarely worth the effort.

## Validation of short codes

`spade check` validates short-code-specific rules in addition to the [standard pipeline validation](/pipelines/validation/) rules:

1. **Grammar.** Every short code must match `@[A-Za-z_][A-Za-z0-9_]*`.
2. **Resolution.** Every short code referenced in `inputs` must be defined as the `id` of some block in the pipeline. (This is enforced via the standard "all referenced IDs exist" check after resolution.)
3. **Uniqueness.** No two blocks may share the same short code. A repeat short code resolves to the same UUID, which then triggers the standard duplicate-id check.
4. **Lockfile validity.** Every binding in the lockfile must be a valid UUIDv7. A malformed lockfile produces an error pointing you to the "delete the lockfile to regenerate" escape hatch.

Failures look like this:

```
Pipeline validation failed with 1 error(s):
  - duplicate block invocation id: 019cf4bc-1111-7000-0000-000000000001
```

If the lockfile is corrupt, you will see:

```
invalid lockfile: binding "@reproject" in pipeline.lock.yaml is not a valid UUID: ...
To regenerate the lockfile from scratch, delete /path/to/pipeline.lock.yaml.
```

## Summary

- Use short codes (`@<identifier>`) instead of UUIDs when writing pipelines by hand or with an LLM.
- The CLI resolves short codes against a sibling lockfile and persists the bindings. Commit the lockfile to version control.
- Short codes appear only in `id` fields and `inputs` references; `args` and the pipeline-level `id` are left alone.
- Deleting the lockfile is the supported way to reset all bindings from scratch.
- Mix UUIDs and short codes freely; the web UI resolves any short codes on upload by minting fresh UUIDs.

## See also

- [Pipeline Format](/pipelines/format/) -- the full reference for the YAML structure.
- [Input References](/pipelines/input-references/) -- bare versus explicit references; type-matching rules.
- [Pipeline Validation](/pipelines/validation/) -- all the rules `spade check` enforces.
- [`spade check`](/cli/check/) and [`spade run`](/cli/run/) -- where lockfile resolution happens.
