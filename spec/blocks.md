# Block Interface and Execution Model

This document defines the **standard block structure**, **execution environment**, and **input/output contracts** for the data processing system. It is intended for block authors (Python, R, or other languages) and for maintainers of the runtime.

The design goals are:

- Language-agnostic execution
- Reproducibility and cacheability
- Clear separation between orchestration and computation
- Robust support for untrusted plugins
- Predictable filesystem semantics (especially for R users)

---

## 1. Core Concepts

### 1.1 What Is a Block?

A **block** is a reusable, isolated unit of computation that:

- Declares its inputs, outputs, and parameters
- Runs as a standalone process
- Reads inputs from a provided working directory
- Writes outputs to a designated output directory

Blocks **do not**:

- Discover inputs dynamically
- Communicate with other blocks directly
- Depend on global filesystem state
- Share a runtime with other blocks

---

## 2. Block Directory Structure

A block is distributed as a directory with the following structure:

```
block/
  block.yaml | block.json
  run.py | run.R
  DESCRIPTION.md
```

Only `block.yaml` and the entrypoint script are required.

---

## 3. Block Manifest (`block.yaml`)

The manifest declares the block’s interface and execution requirements.

### 3.1 Minimal Example

```yaml
id: raster.reproject
version: 1.0.0
language: R
entrypoint: run.R

inputs:
  source:
    type: file
    format: GeoTIFF
  target_crs:
    type: string

outputs:
  raster:
    type: file
    format: GeoTIFF
```

---

### 3.2 Required Fields

| Field        | Description                              |
| ------------ | ---------------------------------------- |
| `id`         | Globally unique block identifier         |
| `version`    | Semantic version of the block            |
| `language`   | Execution language (`python`, `r`, etc.) |
| `entrypoint` | Script executed by the runtime           |
| `inputs`     | Named input declarations                 |
| `outputs`    | Named output declarations                |

---

## 4. Execution Environment

Each block invocation runs in a **fresh working directory** created by the runtime.

```
/<id>/
  params.yaml
  inputs/
  outputs/
  logs/
```

The block’s entrypoint is executed with `/<id>` as the current working directory.

All paths referenced in this document are **relative paths**.

---

## 5. Inputs

### 5.1 Named Inputs

Inputs are addressed by **name**, not by filename.

Example manifest:

```yaml
inputs:
  reference:
    type: file
    format: GeoTIFF
  target:
    type: file
    format: GeoTIFF
```

Runtime layout:

```
params.json

inputs/
  reference/
    data.tif
  target/
    data.tif

output/
```

### 5.2 Input Types

Supported input types include:

| Type                          | Description                              |
| ----------------------------- | ---------------------------------------- |
| `file`                        | Single file input                        |
| `directory`                   | Directory-based input (e.g., shapefiles) |
| `collection`                  | Variable-length collection of items      |
| `string`, `number`, `boolean` | Scalar parameters                        |

---

### 5.3 Collections

For variable numbers of inputs:

```yaml
inputs:
  rasters:
    type: collection
    item_type: file
    format: GeoTIFF
```

Runtime layout:

```
inputs/
  rasters/
    001.tif
    002.tif
    003.tif
```

Ordering is controlled by the runtime and guaranteed to be stable.

---

### 5.4 Scalar Parameters (`params.yaml`)

All scalar inputs are provided via `params.yaml` .

Example:

```yaml
buffer_distance: 30
method: bilinear
normalize: true
```

Blocks should **not** receive parameters via CLI arguments (except the "build" command for bundling the system).

---

## 6. Outputs

Blocks must write all results into the `outputs/` directory.

Example manifest:

```yaml
outputs:
  raster:
    type: file
    format: GeoTIFF
  summary:
    type: json
```

Runtime layout:

```
outputs/
  raster/
    data.tif
  summary.json
```

The runtime collects, hashes, and persists outputs after successful execution.

---

## 7. Metadata (`invocation.yaml`)

The runtime provides a machine-generated metadata file:

```yaml
block:
  id: raster.reproject
  version: 1.0.0
invocation_id: 01HZX...
inputs:
  reference:
    path: inputs/reference/data.tif
    hash: abc123
  target:
    path: inputs/target/data.tif
    hash: def456
```

Blocks may read this file for introspection but should not modify it.


## 8. Caching and Checkpointing

Blocks are cacheable if:

- Outputs depend only on declared inputs and parameters
- No external mutable state is accessed

Cache keys are derived from:

- Block ID + version
- Input content hashes
- `params.yaml`
- Runtime environment hash

---

## 9. What Blocks Must NOT Do

Blocks must not:

- Access files outside the working directory
- Discover inputs dynamically
- Assume original filenames
- Write outside `outputs/`
- Depend on global state

---

## 10. Summary

This block model provides:

- Predictable execution
- Strong isolation
- First-class R support
- Reproducibility and caching
- Clean scaling to distributed systems

Block authors should be able to focus on **domain logic**, not orchestration or plumbing.  
