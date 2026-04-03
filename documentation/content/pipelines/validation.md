+++
title = "Pipeline Validation"
description = "What spade check validates and common errors."
weight = 3
+++

Running `spade check <pipeline.yaml>` validates your pipeline file against a set of rules before execution. This catches structural errors, missing blocks, broken references, and type mismatches early -- before any blocks actually run.

A valid pipeline produces output like:

```
Pipeline 'satellite-reproject' is valid.
  3 blocks, 0 errors.
```

An invalid pipeline produces one or more error messages describing exactly what is wrong and where.

## Validation rules

Spade checks the following seven rules, in order. Each rule is described below with an explanation and an example of a pipeline that violates it.

### Rule 1: Unique invocation IDs

Every block invocation in the pipeline must have a unique `id`. No two blocks may share the same invocation ID.

**Why this matters:** Invocation IDs are used to reference blocks in `inputs` lists. Duplicate IDs would make references ambiguous, and Spade would not know which invocation to use as the data source.

**Example of a violation:**

```yaml
id: 019cf4bc-0000-7000-0000-000000000000
name: duplicate-id-example
version: "1.0"

blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000  # <-- same ID
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"

  - id: 019cf4bc-1111-7000-0000-000000000000  # <-- same ID
    name: raster.reproject
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
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
id: 019cf4bc-0000-7000-0000-000000000000
name: broken-ref-example
version: "1.0"

blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.reproject
    inputs:
      - 019cf4bc-9999-7000-0000-000000000000  # <-- does not exist
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
id: 019cf4bc-0000-7000-0000-000000000000
name: missing-block-example
version: "1.0"

blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.fancy-transform  # <-- not installed
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
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
id: 019cf4bc-0000-7000-0000-000000000000
name: cycle-example
version: "1.0"

blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: raster.process-a
    inputs:
      - 019cf4bc-3333-7000-0000-000000000000  # depends on Block 3
    args: {}

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.process-b
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000  # depends on Block 1
    args: {}

  - id: 019cf4bc-3333-7000-0000-000000000000
    name: raster.process-c
    inputs:
      - 019cf4bc-2222-7000-0000-000000000000  # depends on Block 2
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
id: 019cf4bc-0000-7000-0000-000000000000
name: type-mismatch-example
version: "1.0"

blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: tabular.read-csv
    inputs: []
    args:
      path: "data/measurements.csv"

    # tabular.read-csv produces output of type: file, format: CSV

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.reproject
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
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
id: 019cf4bc-0000-7000-0000-000000000000
name: bad-output-name-example
version: "1.0"

blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: raster.split-bands
    inputs:
      - 019cf4bc-0000-7000-0000-000000000001
    args:
      red_band: 4
      nir_band: 8

    # raster.split-bands declares outputs: "red", "nir"

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.classify
    inputs:
      - block: 019cf4bc-1111-7000-0000-000000000000
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
id: 019cf4bc-0000-7000-0000-000000000000
name: missing-args-example
version: "1.0"

blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"
      # Missing required arg: date_range

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.reproject
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
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

## Tips for fixing validation errors

- **Duplicate IDs**: Generate a new UUIDv7 for one of the conflicting blocks.
- **Broken references**: Check for typos in the invocation ID. Copy-paste the ID directly from the target block.
- **Missing blocks**: Run `spade install <repository>` to install the required collection.
- **Cycles**: Restructure your pipeline so data flows in one direction. If you have a feedback loop in your algorithm, consider implementing it inside a single block rather than across multiple blocks.
- **Type mismatches**: Check the block manifests (`spade check` with no arguments in a collection directory) to confirm the input and output types. You may need a different block or an intermediate conversion step.
- **Bad output names**: Run `spade check` in the upstream block's collection to see its declared outputs, or inspect its `blocks/<name>.yaml` manifest directly.
- **Missing args**: Check the block manifest for required parameters and add them to your `args` map.
