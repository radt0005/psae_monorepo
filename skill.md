# Spade Block Development Skill

You are helping a developer create and work with blocks for the Spade data processing system. Before doing anything, read the relevant spec files in `./spec/` to understand the current system design. The most important files are:

- `spec/blocks.md` -- block manifest format, collection structure, types, execution environment
- `spec/libraries.md` -- runtime library types, handler function patterns, output handling
- `spec/pipeline.md` -- pipeline format, input references, map/reduce in pipelines
- `spec/scheduler.md` -- map/reduce block mechanics, expansion manifests
- `spec/cli.md` -- CLI commands and workflows
- `spec/worker.md` -- block lookup, execution, security, registry

Always read the relevant specs before generating code or manifests to ensure you are using the latest conventions.

---

## Collection Structure

Blocks live in **collections**. The language is detected from the repo root:

| File              | Language         |
| ----------------- | ---------------- |
| `Cargo.toml`      | Rust             |
| `go.mod`          | Go               |
| `pyproject.toml`  | Python           |
| `package.json`    | TypeScript (Bun) |
| *(none of above)* | R                |

Block manifests go in `blocks/<name>.yaml`. Source code goes in the language's standard layout.

---

## Creating a Block Manifest

When the user asks you to create a block, generate a manifest at `blocks/<name>.yaml`. Always read `spec/blocks.md` section 3 for the current field definitions before generating.

The manifest structure is:

```yaml
id: <collection>.<block_name>
version: 1.0.0
kind: standard          # standard | map | reduce
network: false          # true only if the block needs internet access
description: <short description of what the block does>

inputs:
  <input_name>:
    type: <type>        # file | directory | collection | string | number | boolean
    format: <format>    # optional: GeoTIFF, GeoJSON, CSV, Parquet, etc.
    description: <what this input represents>
    item_type: <type>   # only for collection type

outputs:
  <output_name>:
    type: <type>        # file | directory | collection | json | expansion
    format: <format>    # optional
    description: <what this output represents>
    item_type: <type>   # only for collection type

# Optional: override the default entrypoint
# entrypoint: <custom_entrypoint>
```

Rules:
- The `id` should follow the convention `<collection>.<block_name>`
- Every input and output must have a `description`
- Only set `network: true` if the block genuinely needs internet access (LLM APIs, data downloads, etc.)
- Only use `kind: map` or `kind: reduce` for map/reduce blocks (see below)
- Only add `entrypoint` if the default doesn't work (e.g. uv named scripts)
- The filename determines the block name: `blocks/rasterize.yaml` = block `rasterize`

---

## Creating Handler Functions

When creating a block, also create the corresponding entry point source file. The handler function should match the inputs and outputs declared in the manifest.

### Python

```python
from spade import run, RasterFile, VectorFile

def handler(source: RasterFile, boundary: VectorFile) -> RasterFile:
    """Clip a raster to a vector boundary."""
    # processing logic here
    result_path = clip(source.path, boundary.path)
    return RasterFile(path=result_path)

if __name__ == "__main__":
    run(handler)
```

- Place in the package source directory (e.g. `src/<package>/<name>.py`)
- Use spade library types for inputs and return values
- Scalar parameters from `params.yaml` are passed as keyword arguments alongside file inputs
- Always include `if __name__ == "__main__": run(handler)` as an include guard
- The `run()` function handles reading `params.yaml`, scanning `inputs/`, calling the handler, and writing outputs

### R

```r
library(spade)

handler <- function(source, boundary, buffer_distance) {
  # source and boundary are file paths
  # buffer_distance comes from params.yaml
  r <- raster::raster(source)
  # processing logic here
  output_path <- "outputs/result/clipped.tif"
  raster::writeRaster(r, output_path)
}

run(handler)
```

- Place in `R/<name>.R`

### TypeScript

```typescript
import { run, RasterFile, VectorFile } from "spade";

async function handler(source: RasterFile, boundary: VectorFile): Promise<RasterFile> {
  // processing logic here
  const resultPath = await clip(source.path, boundary.path);
  return new RasterFile(resultPath);
}

run(handler);
```

- Place in `src/<name>.ts`
- `run()` may be async

### Rust

```rust
use spade::{run, RasterFile, VectorFile, Result};

fn main() {
    run(|source: RasterFile, boundary: VectorFile| -> Result<RasterFile> {
        // processing logic here
        let result = clip(&source.path, &boundary.path)?;
        Ok(RasterFile::new(result))
    });
}
```

- Rust blocks use closures passed to `run()`
- Each block is a subcommand in the collection binary

### Go

```go
package main

import "github.com/psae/spade"

func handler(source spade.RasterFile, boundary spade.VectorFile) spade.RasterFile {
    // processing logic here
    resultPath := clip(source.Path, boundary.Path)
    return spade.RasterFile{Path: resultPath}
}

func main() {
    spade.Run(handler)
}
```

- Each block is a subcommand in the collection binary

---

## Map and Reduce Blocks

### Map Blocks

Map blocks enumerate a collection and produce an expansion manifest. Set `kind: map` in the manifest:

```yaml
id: mylib.split_tiles
kind: map
network: false
description: Splits a raster collection into individual items for parallel processing

inputs:
  source:
    type: collection
    item_type: file
    format: GeoTIFF
    description: Collection of raster tiles to fan out

outputs:
  manifest:
    type: expansion
    description: Expansion manifest listing items for parallel processing
```

The handler writes an `expansion.yaml` file to `outputs/manifest/`:

```yaml
items:
  - path: inputs/source/tile_001.tif
    key: tile_001
  - path: inputs/source/tile_002.tif
    key: tile_002
```

Item order must be deterministic for caching. See `spec/scheduler.md` for details.

### Reduce Blocks

Reduce blocks collect mapped outputs back into a single result. Set `kind: reduce`:

```yaml
id: mylib.mosaic
kind: reduce
network: false
description: Mosaics a collection of raster tiles into a single raster

inputs:
  tiles:
    type: collection
    item_type: file
    format: GeoTIFF
    description: Raster tiles to mosaic together

outputs:
  mosaic:
    type: file
    format: GeoTIFF
    description: Single mosaiced raster output
```

Reduce blocks always take a `collection` input and can output anything (file, collection, json, etc.).

---

## Type Reference

### File/Directory Types

| Spade Type              | Manifest `type` | Description                     |
| ----------------------- | --------------- | ------------------------------- |
| `File`                  | `file`          | Single file                     |
| `RasterFile`            | `file`          | Raster data (GeoTIFF, etc.)     |
| `VectorFile`            | `file`          | Vector data (GeoJSON, Shapefile) |
| `TabularFile`           | `file`          | Tabular data (CSV, Parquet)     |
| `Directory`             | `directory`     | Directory of files              |
| `RasterFileCollection`  | `collection`    | Collection of raster files      |
| `VectorFileCollection`  | `collection`    | Collection of vector files      |
| `TabularFileCollection` | `collection`    | Collection of tabular files     |

### Scalar Types (via `params.yaml`)

| Manifest `type` | Description              |
| ---------------- | ------------------------ |
| `string`         | String parameter         |
| `number`         | Numeric parameter        |
| `boolean`        | Boolean parameter        |

Scalar types are delivered via `params.yaml`, not the `inputs/` directory.

### Output-Only Types

| Manifest `type` | Description                                      |
| ---------------- | ------------------------------------------------ |
| `json`           | JSON data output                                 |
| `expansion`      | Map expansion manifest (only for `kind: map`)    |

---

## Common Formats

Use these values for the `format` field in manifests:

| Format      | Extensions     | Description                          |
| ----------- | -------------- | ------------------------------------ |
| `GeoTIFF`   | .tif, .tiff    | Raster imagery                       |
| `GeoJSON`   | .geojson, .json| Vector geometries                    |
| `Shapefile` | .shp (+ .dbf, .shx, .prj) | Vector (use `directory` type) |
| `CSV`       | .csv           | Tabular data                         |
| `Parquet`   | .parquet       | Columnar tabular data                |
| `GeoParquet`| .parquet       | Geospatial columnar data             |
| `COG`       | .tif           | Cloud Optimized GeoTIFF              |
| `VRT`       | .vrt           | GDAL Virtual Raster                  |
| `NetCDF`    | .nc            | Multidimensional array data          |
| `JSON`      | .json          | Generic JSON                         |

---

## Working with the CLI

When the user needs to work with the Spade CLI:

- `spade init` -- scaffold a new collection (prompts for language)
- `spade add <name>` -- add a block (creates manifest + entry point)
- `spade check` -- validate all block manifests
- `spade check <pipeline.yaml>` -- validate a pipeline file
- `spade run <pipeline.yaml>` -- run a pipeline locally
- `spade install <git-url>` -- install a block collection
- `spade upload` -- package and upload for cloud deployment

---

## Checklist: Creating a Block

When asked to create a block, follow these steps:

1. **Determine the collection context**: check what language the collection uses (look for `Cargo.toml`, `pyproject.toml`, etc. at the repo root)
2. **Create the manifest**: write `blocks/<name>.yaml` with all required fields, descriptions on every input/output, and the correct `id` convention
3. **Create the entry point**: write the handler function in the language's standard source location
4. **Match the manifest to the handler**: ensure input/output names and types in the manifest correspond exactly to the handler function's parameters and return type
5. **Verify**: if the `spade` CLI is available, suggest running `spade check` to validate

## Checklist: Creating a Pipeline

When asked to create a pipeline, follow these steps:

1. **Read `spec/pipeline.md`** for the current pipeline format
2. **Identify the blocks**: determine which blocks are needed and their dependencies
3. **Wire inputs/outputs**: use bare references when types are unambiguous, explicit references (`block` + `output`) when a dependency has multiple outputs of the same type
4. **Add map/reduce if needed**: insert `kind: map` blocks before fan-out and `kind: reduce` blocks to collect results
5. **Validate**: suggest running `spade check <pipeline.yaml>`
