+++
title = "Pipeline Format"
description = "Complete reference for the YAML pipeline file structure."
weight = 1
+++

This page is the complete reference for the structure of a Spade pipeline YAML file. A pipeline declares a set of block invocations and their dependencies, which together form a directed acyclic graph (DAG) of processing steps.

## Top-level fields

Every pipeline file begins with four top-level fields:

| Field         | Type   | Required | Description |
|---------------|--------|----------|-------------|
| `id`          | string | yes      | A globally unique identifier for the pipeline, in UUIDv7 format |
| `name`        | string | yes      | A human-readable name for the pipeline |
| `version`     | string | yes      | The pipeline version (any string, typically semver) |
| `description` | string | no       | A short description of what the pipeline does |

### The `id` field

Pipeline IDs use [UUIDv7](https://www.ietf.org/rfc/rfc9562.html) format. UUIDv7 is a time-ordered UUID, meaning IDs generated later sort after IDs generated earlier. The format looks like this:

```
019cf4bc-0000-7000-0000-000000000000
```

You can generate a UUIDv7 using the Spade CLI or any UUIDv7 library. The important thing is that the ID is globally unique across all pipelines.

### The `name` field

The name is for human identification. It appears in CLI output, logs, and working directory paths. Use lowercase letters, numbers, and hyphens. For example: `satellite-reproject`, `ndvi-analysis`, or `tile-classification`.

### The `version` field

The version must be a string (wrap numbers in quotes: `"1.0"`). Spade does not enforce a particular versioning scheme, but semantic versioning (e.g., `"1.0.0"`) is recommended.

## The `blocks` list

The `blocks` field is an ordered list of block invocations. Each entry describes one step in the pipeline: which block to run, what data it receives, and what parameters to pass.

```yaml
blocks:
  - id: ...
    name: ...
    inputs: ...
    args: ...
```

The order of blocks in the list does not determine execution order. Spade uses the dependency graph (defined by `inputs`) to determine which blocks can run and when. However, listing blocks in roughly topological order (sources first, sinks last) makes the file easier to read.

## Block invocation fields

Each block invocation has the following fields:

| Field    | Type              | Required | Description |
|----------|-------------------|----------|-------------|
| `id`     | string            | yes      | A unique invocation ID within this pipeline, in UUIDv7 format |
| `name`   | string            | yes      | The block to run, in `collection.block` format |
| `inputs` | list              | yes      | References to upstream block invocations that provide input data |
| `args`   | map (string: any) | no       | Parameters passed to the block at runtime |

### The block `id`

Each block invocation gets its own UUIDv7 identifier, separate from the pipeline ID. This ID must be unique within the pipeline. It is used by other blocks to reference this invocation in their `inputs` lists.

### The block `name`

The `name` field specifies which installed block to run. It uses the format `collection.block`, where `collection` is the name of the block collection and `block` is the name of the specific block within that collection. For example:

- `data.sentinel2` -- the `sentinel2` block from the `data` collection
- `raster.reproject` -- the `reproject` block from the `raster` collection
- `csv-stats.summarize` -- the `summarize` block from the `csv-stats` collection

The block must be installed locally (via `spade install`) before the pipeline can run.

### The `inputs` list

The `inputs` list declares which upstream block invocations feed data into this block. Each entry is a reference to another block's invocation ID. There are two reference styles:

- **Bare reference**: Just the invocation ID string. Spade matches outputs to inputs by type.
- **Explicit reference**: An object with `block` and `output` keys, naming both the source invocation and the specific output.

For source blocks (blocks with no upstream dependencies), use an empty list: `inputs: []`.

See [Input References](/pipelines/input-references/) for full details on both reference styles and the type-matching algorithm.

### The `args` map

The `args` field is a key-value map of parameters passed to the block. At runtime, Spade writes these values into a `params.yaml` file in the block's working directory. The block's handler function reads parameters from this file (the Spade libraries handle this automatically).

Args correspond to the scalar parameters declared in the block's manifest -- inputs of type `string`, `number`, `boolean`, or similar non-file types. For example, if a block manifest declares:

```yaml
inputs:
  target_crs:
    type: string
    description: The target coordinate reference system
  resolution:
    type: number
    description: Output resolution in meters
```

Then the pipeline would pass these as args:

```yaml
args:
  target_crs: "EPSG:4326"
  resolution: 30
```

File-type inputs come from upstream blocks via the `inputs` list, not from `args`. The `args` field is only for scalar parameters.

## How `args` become `params.yaml`

When Spade executes a block invocation, it creates a working directory with the following structure:

```
<working_dir>/
  params.yaml      # Contains the args from the pipeline
  inputs/          # Symlinks to upstream block outputs
  outputs/         # Where the block writes its results
  logs/            # Captured stdout and stderr
```

The `params.yaml` file is a direct YAML serialization of the `args` map. For example, given this pipeline snippet:

```yaml
args:
  target_crs: "EPSG:4326"
  resolution: 30
  overwrite: true
```

The generated `params.yaml` would contain:

```yaml
target_crs: "EPSG:4326"
resolution: 30
overwrite: true
```

The block's handler function receives these values as typed parameters. In Python, for instance, a handler with the signature `def handler(raster: RasterFile, target_crs: str, resolution: int)` would receive `target_crs` as a Python string and `resolution` as a Python integer, loaded automatically from `params.yaml`.

## Complete annotated example

Below is a complete pipeline file with inline comments explaining every field:

```yaml
# -------------------------------------------------------
# Top-level pipeline metadata
# -------------------------------------------------------

# Globally unique pipeline ID (UUIDv7 format).
# Generate one with `spade uuid` or any UUIDv7 library.
id: 019cf4bc-0000-7000-0000-000000000000

# Human-readable name. Appears in CLI output and logs.
name: satellite-reproject

# Pipeline version. Must be a quoted string.
version: "1.0"

# Optional description of the pipeline's purpose.
description: >
  Download Sentinel-2 imagery for a region of interest
  and reproject it to EPSG:4326.

# -------------------------------------------------------
# Block invocations
# -------------------------------------------------------
blocks:

  # ------ Block 1: Download satellite imagery -----------
  - id: 019cf4bc-1111-7000-0000-000000000000
      # Unique invocation ID (UUIDv7) within this pipeline.
      # Other blocks reference this ID to depend on this block.

    name: data.sentinel2
      # Which block to run: the "sentinel2" block from the
      # "data" collection. Must be installed via spade install.

    inputs: []
      # Empty list: this is a source block with no upstream
      # dependencies. It will be among the first blocks to run.

    args:
      # Scalar parameters written to params.yaml at runtime.
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"
        # A WKT polygon defining the area of interest.
      date_range: "2025-01-01/2025-06-01"
        # ISO 8601 date range for the imagery search window.

  # ------ Block 2: Reproject the downloaded raster ------
  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.reproject
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
        # Bare reference to Block 1's invocation ID.
        # Spade will match Block 1's raster output to this
        # block's raster input by type. See "Input References"
        # for details on bare vs. explicit references.

    args:
      target_crs: "EPSG:4326"
        # The coordinate reference system to reproject into.

  # ------ Block 3: Compute NDVI from the reprojected raster
  - id: 019cf4bc-3333-7000-0000-000000000000
    name: raster.ndvi
    inputs:
      - 019cf4bc-2222-7000-0000-000000000000
        # Depends on Block 2 (the reprojected raster).
    args:
      red_band: 4
        # Band index for the red channel.
      nir_band: 8
        # Band index for the near-infrared channel.
```

This pipeline forms the following DAG:

```
data.sentinel2 --> raster.reproject --> raster.ndvi
```

Spade executes `data.sentinel2` first (no dependencies), then `raster.reproject` (depends on Block 1), then `raster.ndvi` (depends on Block 2). Each block's output directory becomes available as input to the next block via symlinks.

## Summary of rules

- The pipeline `id` and every block `id` must be valid UUIDv7 strings.
- All block `id` values must be unique within the pipeline.
- Block `name` values must refer to installed blocks using `collection.block` format.
- `inputs` references must point to `id` values that exist in the same pipeline.
- `args` keys must match the scalar parameter names declared in the block's manifest.
- The dependency graph formed by `inputs` must be acyclic (no circular dependencies).

For validation details, see [Pipeline Validation](/pipelines/validation/).
