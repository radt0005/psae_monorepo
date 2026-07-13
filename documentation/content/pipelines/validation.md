+++
title = "Pipeline Validation"
description = "What spade check validates and common errors."
weight = 4
+++

Running `spade check <pipeline.yaml>` validates your pipeline file against a set of rules before execution. This catches structural errors, missing blocks, broken references, and type mismatches early -- before any blocks actually run.

A valid pipeline produces output like:

```
Pipeline 'satellite-reproject' is valid.
  3 blocks, 0 errors.
```

An invalid pipeline produces one or more error messages describing exactly what is wrong and where.

## Validation rules

Spade checks the following seven core rules, in order, plus two additional rule sets that only apply if your pipeline uses short codes or map/reduce (see [Additional validation for short codes and map/reduce](#additional-validation-for-short-codes-and-map-reduce) below). Each core rule is described below with an explanation and an example of a pipeline that violates it.

### Rule 1: Unique invocation IDs

Every block invocation in the pipeline must have a unique `id`. No two blocks may share the same invocation ID.

**Why this matters:** Invocation IDs are used to reference blocks in `inputs` lists. Duplicate IDs would make references ambiguous, and Spade would not know which invocation to use as the data source.

**Example of a violation:**

```yaml
name: duplicate-id-example
version: "1.0"

blocks:
  - id: "@sentinel2"  # <-- same ID
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"

  - id: "@sentinel2"  # <-- same ID
    name: raster.reproject
    inputs:
      - "@sentinel2"
    args:
      target_crs: "EPSG:4326"
```

**Expected error:**

```
Error: Duplicate block invocation ID '019cf4bc-1111-7000-0000-000000000000'.
  Block 'data.sentinel2' and block 'raster.reproject' share the same ID.
  Each block invocation must have a unique ID.
```

### Rule 2: All referenced IDs exist

Every invocation ID referenced in an `inputs` list must correspond to an actual block invocation in the pipeline. This applies to both bare references and the `block` key in explicit references.

**Why this matters:** A reference to a non-existent ID means the block is expecting data from a step that does not exist. This is usually caused by a typo or a deleted block.

**Example of a violation:**

```yaml
name: broken-ref-example
version: "1.0"

blocks:
  - id: "@sentinel2"
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"

  - id: "@reproject"
    name: raster.reproject
    inputs:
      - "@deleted-block"  # <-- does not exist
    args:
      target_crs: "EPSG:4326"
```

**Expected error:**

```
Error: Block 'raster.reproject' (019cf4bc-2222-7000-0000-000000000000)
  references invocation ID '019cf4bc-9999-7000-0000-000000000000',
  but no block with that ID exists in this pipeline.
```

### Rule 3: Block names refer to installed blocks

Every `name` field in the `blocks` list must refer to a block that is installed in the local Spade environment. The name uses `collection.block` format, and both the collection and the specific block must be present.

**Why this matters:** If Spade cannot find the block definition, it cannot determine the block's inputs, outputs, or how to execute it.

**Example of a violation:**

```yaml
name: missing-block-example
version: "1.0"

blocks:
  - id: "@sentinel2"
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"

  - id: "@fancy-transform"
    name: raster.fancy-transform  # <-- not installed
    inputs:
      - "@sentinel2"
    args: {}
```

**Expected error:**

```
Error: Block 'raster.fancy-transform' is not installed.
  Referenced by invocation '019cf4bc-2222-7000-0000-000000000000'.
  Run `spade install <repository>` to install the collection first.
```

### Rule 4: Dependency graph is acyclic

The dependency graph formed by `inputs` references must be a directed acyclic graph (DAG). In other words, there must be no circular dependencies where Block A depends on Block B, which depends on Block C, which depends on Block A.

**Why this matters:** Spade executes blocks by running dependencies first. A cycle means no block in the cycle can run before the others, creating a deadlock. Processing pipelines are inherently feedforward -- data flows from sources to sinks.

**Example of a violation:**

```yaml
name: cycle-example
version: "1.0"

blocks:
  - id: "@process-a"
    name: raster.process-a
    inputs:
      - "@process-c"  # depends on Block 3
    args: {}

  - id: "@process-b"
    name: raster.process-b
    inputs:
      - "@process-a"  # depends on Block 1
    args: {}

  - id: "@process-c"
    name: raster.process-c
    inputs:
      - "@process-b"  # depends on Block 2
    args: {}
```

**Expected error:**

```
Error: Dependency cycle detected.
  raster.process-a (019cf4bc-1111-7000-0000-000000000000)
    -> raster.process-b (019cf4bc-2222-7000-0000-000000000000)
    -> raster.process-c (019cf4bc-3333-7000-0000-000000000000)
    -> raster.process-a (019cf4bc-1111-7000-0000-000000000000)
  Pipelines must form a directed acyclic graph (DAG).
```

### Rule 5: Input/output type compatibility

When two blocks are connected (one listed in the other's `inputs`), the upstream block's output types must be compatible with the downstream block's input types. For bare references, at least one unambiguous type match must exist. For explicit references, the named output must be type-compatible with an available input.

**Why this matters:** Type checking prevents runtime failures where a block receives data it cannot process. For instance, connecting a block that produces a JSON file to a block that expects a raster file would fail at runtime.

**Example of a violation:**

```yaml
name: type-mismatch-example
version: "1.0"

blocks:
  - id: "@read-csv"
    name: tabular.read-csv
    inputs: []
    args:
      path: "data/measurements.csv"

    # tabular.read-csv produces output of type: file, format: CSV

  - id: "@reproject"
    name: raster.reproject
    inputs:
      - "@read-csv"
    args:
      target_crs: "EPSG:4326"

    # raster.reproject expects input of type: file, format: GeoTIFF
```

**Expected error:**

```
Error: Type mismatch between 'tabular.read-csv' and 'raster.reproject'.
  Output 'data' (type: file, format: CSV) from 'tabular.read-csv'
  is not compatible with any input of 'raster.reproject'
  (expects type: file, format: GeoTIFF).
```

### Rule 6: Named outputs match declarations

When an explicit reference uses the `output` key, the named output must actually exist in the upstream block's manifest. If the upstream block does not declare an output with that name, validation fails.

**Why this matters:** An explicit reference to a non-existent output is an error -- there would be no data to wire. This is often caused by a typo in the output name.

**Example of a violation:**

```yaml
name: bad-output-name-example
version: "1.0"

blocks:
  - id: "@split-bands"
    name: raster.split-bands
    inputs:
      - "@source"
    args:
      red_band: 4
      nir_band: 8

    # raster.split-bands declares outputs: "red", "nir"

  - id: "@classify"
    name: raster.classify
    inputs:
      - block: "@split-bands"
        output: blue  # <-- "blue" is not a declared output
    args:
      threshold: 0.3
```

**Expected error:**

```
Error: Block 'raster.split-bands' (019cf4bc-1111-7000-0000-000000000000)
  does not have an output named 'blue'.
  Available outputs: red, nir.
  Referenced by 'raster.classify' (019cf4bc-2222-7000-0000-000000000000).
```

### Rule 7: Required args are present

Every required scalar parameter declared in a block's manifest must have a corresponding entry in the `args` map of the pipeline invocation. Parameters that have default values in the manifest are optional and may be omitted.

**Why this matters:** If a required parameter is missing, the block handler will fail at runtime when it tries to read the parameter from `params.yaml`.

**Example of a violation:**

```yaml
name: missing-args-example
version: "1.0"

blocks:
  - id: "@sentinel2"
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"
      # Missing required arg: date_range

  - id: "@reproject"
    name: raster.reproject
    inputs:
      - "@sentinel2"
    args: {}
    # Missing required arg: target_crs
```

**Expected error:**

```
Error: Block 'data.sentinel2' (019cf4bc-1111-7000-0000-000000000000)
  is missing required argument 'date_range'.

Error: Block 'raster.reproject' (019cf4bc-2222-7000-0000-000000000000)
  is missing required argument 'target_crs'.
```

## Additional validation for short codes and map/reduce

These rules only apply to pipelines that use the relevant feature -- they run in addition to the seven core rules above.

### Short codes

If a pipeline uses `@`-prefixed short codes instead of UUIDs, `spade check` additionally verifies:

1. **Grammar** -- every short code matches `@[A-Za-z_][A-Za-z0-9_]*`.
2. **Resolution** -- every short code referenced in `inputs` is defined as the `id` of some block in the pipeline.
3. **Uniqueness** -- no two blocks share the same short code (a repeat resolves to the same UUID, which then trips Rule 1 above).
4. **Lockfile validity** -- every binding in the sibling `.lock.yaml` is a valid UUIDv7, and every bound short code that's still referenced in the source exists.

See [Short Codes and Hand-Authoring](/pipelines/short-codes/#validation-of-short-codes) for the full detail and error message examples.

### Map/reduce

If a pipeline contains `kind: map` or `kind: reduce` blocks, `spade check` additionally verifies:

1. **Map blocks output `expansion`** -- a `kind: map` block must declare at least one output of type `expansion`.
2. **Reduce blocks accept `collection`** -- a `kind: reduce` block must declare at least one input of type `collection`.
3. **Every map context is closed by a reduce** -- a fan-out that never reaches a matching `kind: reduce` block is rejected.
4. **Contexts are well-nested** -- a block may not combine the outputs of two sibling map contexts unless at least one has already been closed by its reduce.
5. **Nesting depth does not exceed 4 levels** -- since invocation counts multiply at each nesting level, this bounds worst-case fan-out.

See [Map/Reduce Pipelines](/pipelines/map-reduce-pipelines/) and [Nested map/reduce](/concepts/map-reduce/#nested-mapreduce) for the full mechanics.

## Summary of validation rules

| # | Rule | What it checks |
|---|------|----------------|
| 1 | Unique invocation IDs | No two blocks share the same `id` |
| 2 | Referenced IDs exist | Every ID in `inputs` corresponds to a block in the pipeline |
| 3 | Block names installed | Every `name` refers to a locally installed block |
| 4 | Acyclic graph | The dependency graph has no cycles |
| 5 | Type compatibility | Connected blocks have compatible input/output types |
| 6 | Named outputs exist | Explicit `output` references match declared outputs |
| 7 | Required args present | All required scalar parameters are provided in `args` |
| -- | Short codes (if used) | Grammar, resolution, uniqueness, lockfile validity -- see above |
| -- | Map/reduce (if used) | Output/input types, context closure, well-nestedness, depth ≤ 4 -- see above |

## Tips for fixing validation errors

- **Duplicate IDs**: Generate a new UUIDv7 for one of the conflicting blocks. If you used the same short code (e.g. `"@foo"`) on two blocks, rename one -- short codes resolve to the same UUID, so duplicates trigger this rule.
- **Broken references**: Check for typos in the invocation ID or short code. Copy-paste the ID or short code directly from the target block.
- **Missing blocks**: Run `spade install <repository>` to install the required collection.
- **Cycles**: Restructure your pipeline so data flows in one direction. If you have a feedback loop in your algorithm, consider implementing it inside a single block rather than across multiple blocks.
- **Type mismatches**: Check the block manifests (`spade check` with no arguments in a collection directory) to confirm the input and output types. You may need a different block or an intermediate conversion step.
- **Bad output names**: Run `spade check` in the upstream block's collection to see its declared outputs, or inspect its `blocks/<name>.yaml` manifest directly.
- **Missing args**: Check the block manifest for required parameters and add them to your `args` map.
- **Corrupt lockfile**: If `spade check` reports `invalid lockfile: ...`, delete the sibling `<pipeline-stem>.lock.yaml` to regenerate bindings from scratch. See [Short Codes and Hand-Authoring](/pipelines/short-codes/#lockfile-rules) for the full set of lockfile rules.
