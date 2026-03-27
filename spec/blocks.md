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

## 2. Block Collections

Blocks are organized into **collections** -- repositories that group related blocks together and share a common language and build system.  The collection is the unit of distribution and installation.

### 2.1 Collection Structure

The language of a collection is detected from the repository root:

| File              | Language   |
| ----------------- | ---------- |
| `Cargo.toml`      | Rust       |
| `go.mod`          | Go         |
| `pyproject.toml`  | Python     |
| `package.json`    | TypeScript (Bun) |
| *(none of above)* | R          |

Block manifests live in a `blocks/` directory, with the filename matching the block name:

```
gdal-tools/
  Cargo.toml                # → language: Rust
  src/                       # standard Rust source layout
    lib.rs
    rasterize.rs
    reproject.rs
    clip.rs
  blocks/
    rasterize.yaml           # block manifest for "rasterize"
    reproject.yaml           # block manifest for "reproject"
    clip.yaml                # block manifest for "clip"
  docs/                      # optional extended documentation
    rasterize.md
    reproject.md
```

For Python:
```
my-python-blocks/
  pyproject.toml
  src/
    my_blocks/
      rasterize.py
      reproject.py
  blocks/
    rasterize.yaml
    reproject.yaml
```

For R:
```
my-r-blocks/
  renv.lock
  R/
    rasterize.R
    reproject.R
  blocks/
    rasterize.yaml
    reproject.yaml
```

No `collection.yaml` is needed.  The CLI discovers everything by scanning the repository: language from the root manifest, blocks from `blocks/*.yaml`, and collection name from the language's own manifest (e.g. the `name` field in `pyproject.toml` or `Cargo.toml`).

### 2.2 Installed Layout

Collections are installed to `~/.spade/blocks/<collection>/<version>/`.  For compiled languages (Rust, Go, Bun), the collection produces a single binary with subcommands.  For interpreted languages, the collection is installed as a package (Python) or directory (R).

---

## 3. Block Manifest

Each block has a YAML manifest in the collection’s `blocks/` directory.  The filename determines the block name (e.g. `blocks/rasterize.yaml` defines the `rasterize` block).

### 3.1 Full Example

```yaml
id: gdal.rasterize
version: 1.0.0
kind: standard
network: false
description: Converts vector geometries to raster format using GDAL
entrypoint: rasterize    # optional override; defaults to filename stem

inputs:
  vectors:
    type: file
    format: GeoJSON
    description: Vector file containing the geometries to rasterize
  resolution:
    type: number
    description: Output pixel size in CRS units
  burn_value:
    type: number
    description: Value to assign to pixels covered by a geometry

outputs:
  raster:
    type: file
    format: GeoTIFF
    description: Rasterized output at the requested resolution
```

### 3.2 Fields

| Field         | Required | Description                              |
| ------------- | -------- | ---------------------------------------- |
| `id`          | Yes      | Globally unique block identifier (conventionally `<collection>.<block>`) |
| `version`     | Yes      | Semantic version of the block            |
| `kind`        | No       | Block kind: `standard`, `map`, or `reduce` (default `standard`). See `scheduler.md` for map/reduce semantics. |
| `network`     | No       | Whether the block requires network access (default `false`) |
| `description` | No       | Short human-readable description of what the block does |
| `entrypoint`  | No       | Override for the entrypoint. Defaults to the filename stem (e.g. `rasterize.yaml` → subcommand `rasterize` for compiled languages, or script `rasterize.py`/`rasterize.R` for interpreted). Useful for non-standard entry points such as named scripts in `uv`. |
| `inputs`      | Yes      | Named input declarations (see section 5) |
| `outputs`     | Yes      | Named output declarations (see section 6) |

### 3.3 Input and Output Declarations

Each named input and output supports the following fields:

| Field         | Required | Description                              |
| ------------- | -------- | ---------------------------------------- |
| `type`        | Yes      | The data type (see sections 5.2 and 6.1) |
| `format`      | No       | File format hint (e.g. `GeoTIFF`, `GeoJSON`, `CSV`) |
| `description` | No       | Human-readable description of what this input/output represents |
| `item_type`   | No       | For `collection` types: the type of each item in the collection |

---

## 4. Execution Environment

Each block invocation runs in a **fresh working directory** created by the runtime.

```
/work/
  invocation.yaml
  params.yaml
  inputs/
  outputs/
  logs/
```

The block’s entrypoint is executed with `/work` as the current working directory.

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
    description: Reference raster to align against
  target:
    type: file
    format: GeoTIFF
    description: Raster to be aligned
```

Runtime layout:

```
params.yaml

inputs/
  reference/
    data.tif
  target/
    data.tif

outputs/
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
    description: Collection of raster tiles to process
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

All scalar inputs are provided via `params.yaml`.

Example:

```yaml
buffer_distance: 30
method: bilinear
normalize: true
```

Blocks should **not** receive parameters via CLI arguments.

---

## 6. Outputs

Blocks must write all results into the `outputs/` directory.

Example manifest:

```yaml
outputs:
  raster:
    type: file
    format: GeoTIFF
    description: Reprojected raster in the target CRS
  summary:
    type: json
    description: Processing metadata and statistics
```

Runtime layout:

```
outputs/
  raster/
    data.tif
  summary/
    summary.json
```

Each output is placed in a subdirectory matching its declared name, mirroring the convention used for inputs.  This ensures a consistent and predictable layout regardless of output type.

### 6.1 Output Types

| Type         | Description                                                                 |
| ------------ | --------------------------------------------------------------------------- |
| `file`       | Single file output                                                          |
| `directory`  | Directory-based output                                                      |
| `collection` | Variable-length collection of items                                         |
| `json`       | JSON data file                                                              |
| `expansion`  | Map expansion manifest (only valid for `kind: map` blocks). See section 6.2 |

### 6.2 Expansion Outputs (Map Blocks)

Blocks with `kind: map` produce an `expansion` output -- a YAML manifest listing items for the scheduler to fan out over.  The manifest is written to the output subdirectory as `expansion.yaml`:

```yaml
# outputs/manifest/expansion.yaml
items:
  - path: inputs/source/tile_001.tif
    key: tile_001
  - path: inputs/source/tile_002.tif
    key: tile_002
  - path: inputs/source/tile_003.tif
    key: tile_003
```

The `path` field points to the file relative to the map block's working directory.  The `key` field is a human-readable identifier for the item.  The item order must be **deterministic** for a given input to support caching.

See `scheduler.md` for full map/reduce semantics.

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

---

## 8. Language-Specific Notes

### 8.1 R Blocks

- Assume working directory semantics
- Use relative paths only
- Activate `renv` inside the block if required

Example:

```r
library(yaml)
params <- read_yaml("params.yaml")
r <- raster::raster("inputs/source/data.tif")
```

### 8.2 Python Blocks

- Use standard file I/O
- Virtual environments are managed externally

Example:

```python
import yaml
with open("params.yaml") as f:
    params = yaml.safe_load(f)
```

---

## 9. Caching and Checkpointing

Blocks are cacheable if:

- Outputs depend only on declared inputs and parameters
- No external mutable state is accessed

Cache keys are derived from:

- Block ID + version
- Input content hashes
- `params.yaml`
- Runtime environment hash

---

## 10. Logging

The runtime captures `stdout` and `stderr` from the block process and writes them to the `logs/` directory.  Block authors should use standard output mechanisms for logging (`print` in Python, `cat`/`message` in R, `console.log` in TypeScript, etc.).

---

## 11. Error Handling

If a block exits with a non-zero exit code, the runtime treats the invocation as failed.  The scheduler will **halt the entire pipeline** -- no downstream blocks will be executed.  Logs from the failed block are preserved for debugging.

---

## 12. What Blocks Must NOT Do

Blocks must not:

- Access files outside the working directory
- Discover inputs dynamically
- Assume original filenames
- Write outside `outputs/`
- Depend on global state
- Access the network (unless `network: true` is declared in `block.yaml`)

---

## 13. Summary

This block model provides:

- Predictable execution
- Strong isolation
- First-class R support
- Reproducibility and caching
- Clean scaling to distributed systems

Block authors should be able to focus on **domain logic**, not orchestration or plumbing.