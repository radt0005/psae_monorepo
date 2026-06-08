+++
title = "Pipelines"
description = "Declarative YAML workflows connecting blocks into a directed acyclic graph."
weight = 3
+++

A **pipeline** is a YAML file that describes a workflow as a series of block invocations connected by data dependencies. Spade reads this file, determines the order blocks need to run, and executes them, passing data from one block's outputs to the next block's inputs.

## How pipelines work

A pipeline defines a **directed acyclic graph** (DAG) of block invocations. Each node in the graph is a block invocation, and each edge represents data flowing from one block's output to another block's input. "Directed" means data flows in one direction (from producer to consumer). "Acyclic" means there are no loops: a block cannot depend on its own output, directly or indirectly.

Spade uses this graph structure to determine:

- **Execution order** — Which blocks must run before which other blocks
- **Parallelism** — Which blocks can run at the same time (blocks with no dependencies on each other)
- **Data routing** — How to connect outputs to inputs

## Pipeline structure

Here is a simple pipeline with three blocks:

```yaml
name: ndvi-analysis
version: "1.0"
description: Download satellite imagery, compute NDVI, and generate a report

blocks:
  - id: "@source"
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-105.5 40.0, -105.0 40.0, -105.0 40.5, -105.5 40.5, -105.5 40.0))"
      date_range: "2025-06-01/2025-09-01"

  - id: "@ndvi"
    name: raster.ndvi
    inputs:
      - "@source"
    args: {}

  - id: "@report"
    name: report.summary
    inputs:
      - "@ndvi"
    args:
      format: html
```

Block IDs use `@`-prefixed **short codes** (`@source`, `@ndvi`, `@report`). Short codes are the recommended form for hand-authored pipelines — they are readable, easy to type, and the CLI resolves them to stable UUIDv7s automatically via a sibling lockfile. The pipeline-level `id` is omitted; the CLI generates one at run time.

### Pipeline-level fields

| Field | Required | Description |
|-------|----------|-------------|
| `id` | No | A unique identifier for the pipeline (UUIDv7 format). Omit for hand-authored pipelines — the CLI generates one at run time. |
| `name` | Yes | A human-readable name for the pipeline. |
| `version` | Yes | The pipeline version string. |
| `description` | No | A description of what the pipeline does. |
| `blocks` | Yes | An ordered list of block invocations. |

### Block invocation fields

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | A unique invocation ID (UUIDv7 format, or a `@<identifier>` short code for hand-authored pipelines -- see [Short Codes](/pipelines/short-codes/)). Must be unique within the pipeline. |
| `name` | Yes | The block to run, in `<collection>.<block>` format. |
| `inputs` | Yes | List of upstream invocation IDs or explicit references. See [Input Resolution](/concepts/input-resolution/). |
| `args` | No | Key-value parameters passed to the block via `params.yaml`. |

## Data flow between blocks

When a block lists another block's invocation ID in its `inputs`, Spade connects the upstream block's outputs to the downstream block's inputs. In the example above:

1. `data.sentinel2` runs first because it has no inputs (`inputs: []`)
2. `raster.ndvi` waits for `data.sentinel2` to finish, then receives its output
3. `report.summary` waits for `raster.ndvi` to finish, then receives its output

Spade automatically matches outputs to inputs by comparing types and formats. If the upstream block produces a `file` output with format `GeoTIFF` and the downstream block expects a `file` input with format `GeoTIFF`, Spade connects them. For more complex cases where automatic matching is ambiguous, you can use explicit references. See [Input Resolution](/concepts/input-resolution/) for the full details.

## Dependency resolution and parallel execution

Spade analyzes the dependency graph before execution begins. A block is ready to run as soon as **all** of its upstream dependencies have completed successfully. Blocks that do not depend on each other can run **in parallel**.

Consider this pipeline:

```yaml
blocks:
  - id: "@imagery"
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-105.5 40.0, -105.0 40.0, -105.0 40.5, -105.5 40.5, -105.5 40.0))"

  - id: "@elevation"
    name: data.dem
    inputs: []
    args:
      region: "POLYGON((-105.5 40.0, -105.0 40.0, -105.0 40.5, -105.5 40.5, -105.5 40.0))"

  - id: "@composite"
    name: raster.composite
    inputs:
      - "@imagery"
      - "@elevation"
    args: {}
```

Here, `data.sentinel2` and `data.dem` both have no upstream dependencies, so they run **in parallel**. Once both have completed, `raster.composite` receives their outputs and runs.

## Validation rules

Before running a pipeline, Spade validates it against several rules:

- **Unique IDs** — Every block invocation must have a unique `id` within the pipeline. Duplicate IDs are rejected.
- **Valid references** — Every invocation ID listed in an `inputs` array must refer to another block in the same pipeline. References to nonexistent blocks are rejected.
- **No cycles** — The dependency graph must be acyclic. If block A depends on block B and block B depends on block A (directly or through a chain), the pipeline is rejected.
- **Type compatibility** — Input and output types must be compatible. For example, a block expecting a `file` input cannot receive a `string` output. Spade checks this during validation and reports mismatches.
- **Installed blocks** — Every block referenced by `name` must be installed locally. Missing blocks are reported with installation instructions.

Run validation manually with:

```bash
spade check my-pipeline.yaml
```

## Running a pipeline

Execute a pipeline locally:

```bash
spade run my-pipeline.yaml
```

Spade resolves dependencies, schedules blocks, and executes them in the correct order. See [Execution Model](/concepts/execution-model/) for details on how scheduling, sandboxing, and caching work.
