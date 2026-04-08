# Authoring Spade Blocks

A **block** is a reusable, isolated unit of computation. It runs as its own subprocess in a fresh working directory, reads its inputs from `inputs/<name>/` and `params.yaml`, and writes outputs to `outputs/<name>/`. Blocks are organized into **collections** — one collection per language, distributed as a single repository.

This document covers everything needed to create a block: collection layout, the manifest format, the runtime execution environment, the handler templates for each supported language, and the rules for `kind: map` / `kind: reduce` blocks.

---

## Collection layout

A collection is detected by language from the file at the repository root:

| Root file        | Language          |
| ---------------- | ----------------- |
| `Cargo.toml`     | Rust              |
| `go.mod`         | Go                |
| `pyproject.toml` | Python            |
| `package.json`   | TypeScript (Bun)  |
| *(none)*         | R                 |

Block manifests live in `blocks/<name>.yaml`. The collection name comes from the language's own manifest (the `name` field in `Cargo.toml`/`pyproject.toml`/`package.json`, or the directory name for Go and R). There is no `collection.yaml`.

```
my-collection/
  Cargo.toml | go.mod | pyproject.toml | package.json | renv.lock
  blocks/
    rasterize.yaml
    reproject.yaml
  src/  | R/  | (language source layout)
```

Use `spade init --language <lang>` to scaffold a new collection and `spade add <name>` to add a block. Both are documented in `cli.md`.

---

## Manifest format

```yaml
id: <collection>.<block>          # required, dotted; e.g. gdal.rasterize
version: 1.0.0                    # required, semver
kind: standard                    # standard | map | reduce  (default: standard)
network: false                    # default false; only true if the block needs the network
description: Short human-readable summary of what this block does
entrypoint: rasterize             # optional override; defaults to filename stem

inputs:
  <name>:
    type: file                    # file | directory | collection | string | number | boolean
    format: GeoTIFF               # optional file format hint (GeoTIFF, GeoJSON, CSV, Parquet, etc.)
    description: What this input represents
    item_type: file               # only for type: collection — the type of each item

outputs:
  <name>:
    type: file                    # file | directory | collection | json | expansion
    format: GeoTIFF
    description: What this output represents
    item_type: file               # only for type: collection
```

Field rules:

- **`id`** must follow `<collection>.<block>` so the registry can look up the right collection.
- **`version`** is the block's own semantic version. The collection version comes from the language manifest and is independent.
- **`kind`** defaults to `standard`. Use `map` to fan out a collection, `reduce` to gather mapped outputs back. See the map/reduce section below.
- **`network`** defaults to `false`. The runtime sandbox blocks network access unless this is `true`. Set it only when the block actually needs it (e.g. calls an LLM API, downloads data).
- **`entrypoint`** defaults to the filename stem. Override only when the language toolchain needs a different name (most often for `uv` named scripts in Python).
- **Every input and output should have a `description`.** The web UI displays them when wiring pipelines, and they make blocks discoverable. `spade check` does not enforce this, but blocks without descriptions are hard for users to use.

The filename determines the block name: `blocks/rasterize.yaml` defines a block named `rasterize`. Don't put `id` and filename out of sync.

---

## Execution environment

The runtime creates a fresh working directory per invocation and runs the block with that directory as `cwd`:

```
<work>/
  invocation.yaml      # machine-generated metadata; read-only for the block
  params.yaml          # scalar parameters from the pipeline's args
  inputs/
    <input_name>/      # one subdirectory per declared input
      data.tif         # the symlinked file (or files for collection inputs)
  outputs/             # the block writes results into matching subdirectories
  logs/                # stdout/stderr captured by the worker
```

What the block must **not** do:

- Access files outside the working directory.
- Discover inputs dynamically (always go through `inputs/<declared_name>/`).
- Assume original filenames — the worker symlinks files in, names may differ.
- Write outside `outputs/`.
- Use the network unless `network: true` is declared.
- Depend on global state.

A non-zero exit code halts the entire pipeline. Logs from the failed block are preserved.

---

## Handler templates by language

The runtime libraries (`libs/<lang>/`) handle the boring parts: loading `params.yaml`, scanning `inputs/`, calling the handler, and writing return values into `outputs/<name>/`. Use them — do not roll your own input/output plumbing.

The library uses **parameter names** to find inputs: a function parameter named `source` will be filled from `inputs/source/` (for typed file/directory/collection arguments) or from `params.yaml` (for scalars). Output names come from the block manifest, matched to the keys of a returned dict / fields of a struct, or fall back to the type's default output name when there's a single output.

Pick the section that matches the collection's language.

### Python

Source location: `src/<package>/<name>.py`. The package directory is the one created by `spade init --language python` under `src/`.

```python
"""Block: clip a raster to a vector boundary."""
from spade import run, RasterFile, VectorFile


def handler(source: RasterFile, boundary: VectorFile, buffer_distance: float) -> RasterFile:
    # source.path and boundary.path are the input file paths.
    # buffer_distance is a scalar from params.yaml.
    result_path = clip(source.path, boundary.path, buffer_distance)
    return RasterFile(path=result_path)


if __name__ == "__main__":
    run(handler)
```

Notes:
- Always wrap `run(handler)` in `if __name__ == "__main__":` so the handler stays importable for tests.
- For multiple outputs, return a dict keyed by output name: `return {"raster": RasterFile(...), "summary": JsonFile(...)}`.
- Type hints determine how each parameter is constructed. Subclasses of `File` get a single file from `inputs/<name>/`; `Directory` gets the subdirectory itself; `FileCollection` subclasses get every file in the subdirectory.

### R

Source location: `R/<name>.R`.

```r
library(spade)

handler <- function(source, boundary, buffer_distance) {
  # source and boundary are S4 objects with @path
  # buffer_distance is a numeric from params.yaml
  r <- terra::rast(source@path)
  v <- terra::vect(boundary@path)
  result <- terra::crop(r, terra::buffer(v, buffer_distance))
  out_path <- "result.tif"
  terra::writeRaster(result, out_path, overwrite = TRUE)
  RasterFile(out_path)
}

run(handler)
```

Notes:
- The runtime activates `renv` if present. Declare R dependencies via `renv.lock` and (optionally) a `setup.R` script.
- File-typed arguments are S4 objects (`File`, `RasterFile`, `VectorFile`, …); access the path with `@path`.
- For multiple outputs, return a named list: `list(raster = RasterFile("out.tif"), stats = JsonFile("stats.json"))`.

### TypeScript (Bun)

Source location: `src/<name>.ts`.

```typescript
import { run, RasterFile, VectorFile, spadeBlock } from "spade";

const handler = spadeBlock({
  inputs: { source: RasterFile, boundary: VectorFile, buffer_distance: "number" },
  output: RasterFile,
  description: "Clip a raster to a vector boundary.",
})((args: { source: RasterFile; boundary: VectorFile; buffer_distance: number }): RasterFile => {
  const resultPath = clip(args.source.path, args.boundary.path, args.buffer_distance);
  return new RasterFile(resultPath);
});

run(handler);
```

Notes:
- TypeScript erases types at runtime, so the library uses an explicit `spadeBlock({...})` decorator to register input/output metadata. Without it, `run` cannot match parameter names to types.
- Use `"string"`, `"number"`, `"boolean"` for scalar parameters in the metadata block.
- For multiple outputs, return an object keyed by output name and list each one in `output` (or use the manifest to drive output names).

### Rust

Source location: `src/<name>.rs`, registered in `src/lib.rs` and dispatched as a subcommand from `src/main.rs`.

```rust
use spade::{run, Args, RasterFile, Result};

pub fn entry() {
    run(|args: Args| -> Result<RasterFile> {
        let source: RasterFile = args.input("source")?;
        let boundary: spade::VectorFile = args.input("boundary")?;
        let buffer_distance: f64 = args.param("buffer_distance")?;

        let result_path = clip(&source.path, &boundary.path, buffer_distance)?;
        Ok(RasterFile::new(result_path))
    });
}
```

For multiple outputs use `spade::Outputs`:

```rust
use spade::{run, Args, Outputs, RasterFile, JsonFile, Result};

pub fn entry() {
    run(|args: Args| -> Result<Outputs> {
        let source: RasterFile = args.input("source")?;
        let mut outputs = Outputs::new();
        outputs.add("raster", RasterFile::new("result.tif"));
        outputs.add("stats", JsonFile::new("stats.json"));
        Ok(outputs)
    });
}
```

The collection compiles to a single binary; each block is a subcommand dispatched from `main.rs` based on `args[1]`. After creating a block module, register it in `main.rs`'s subcommand match and re-export from `lib.rs`.

### Go

Source location: `<name>.go` at the package root.

```go
package main

import "spade"

func rasterize(args *spade.Args) (spade.RasterFile, error) {
    var source spade.RasterFile
    if err := args.Input("source", &source); err != nil {
        return spade.RasterFile{}, err
    }
    var resolution float64
    if err := args.Param("resolution", &resolution); err != nil {
        return spade.RasterFile{}, err
    }

    resultPath, err := doRasterize(source.Path, resolution)
    if err != nil {
        return spade.RasterFile{}, err
    }
    return spade.NewRasterFile(resultPath), nil
}
```

Then dispatch it from `main.go`:

```go
package main

import (
    "fmt"
    "os"
    "spade"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Fprintln(os.Stderr, "usage: <collection> <block> [...]")
        os.Exit(2)
    }
    switch os.Args[1] {
    case "rasterize":
        spade.Run(rasterize)
    default:
        fmt.Fprintf(os.Stderr, "unknown block: %s\n", os.Args[1])
        os.Exit(2)
    }
}
```

For multiple outputs, return a struct that satisfies `spade.IntoOutput`, or use the library's grouped-output helpers.

---

## Map and reduce blocks

Map and reduce express parallel "for each" operations across the items of a collection. They are normal blocks with `kind: map` or `kind: reduce` in their manifest. The scheduler reads `kind` and arranges fan-out/fan-in automatically — no special wiring is needed in the pipeline.

### Map blocks (`kind: map`)

A map block reads a collection and writes an **expansion manifest** listing the items the scheduler should fan out over. The manifest is a YAML file at `outputs/<output_name>/expansion.yaml`:

```yaml
items:
  - path: inputs/source/tile_001.tif
    key: tile_001
  - path: inputs/source/tile_002.tif
    key: tile_002
  - path: inputs/source/tile_003.tif
    key: tile_003
```

- `path` is relative to the map block's working directory.
- `key` is a stable, human-readable identifier for the item.
- **Item order must be deterministic** for a given input collection — the cache depends on it.

A typical map block manifest:

```yaml
id: mylib.split_tiles
version: 1.0.0
kind: map
description: Enumerates tiles from a raster collection for parallel processing

inputs:
  source:
    type: collection
    item_type: file
    format: GeoTIFF
    description: Collection of raster tiles to fan out over

outputs:
  manifest:
    type: expansion
    description: Expansion manifest listing each tile as a separate item
```

The core library ships `base.map_files` for the common case of enumerating files in a collection — prefer it over rolling a custom map block when the inputs are already a file collection.

### Reduce blocks (`kind: reduce`)

A reduce block always takes a `collection` input and can produce any output type. The scheduler waits for all N mapped invocations of the upstream block to finish, then presents their outputs as a single collection in `inputs/<name>/`.

```yaml
id: mylib.mosaic
version: 1.0.0
kind: reduce
description: Mosaics a collection of raster tiles into a single raster

inputs:
  tiles:
    type: collection
    item_type: file
    format: GeoTIFF
    description: Raster tiles produced by the upstream mapped block

outputs:
  mosaic:
    type: file
    format: GeoTIFF
    description: Single mosaiced raster
```

### Map context, broadcasting, and nesting

When a `kind: map` block produces N items, every downstream block in the pipeline (until a `kind: reduce`) is invoked N times. A block inside that map context may also depend on a non-mapped block (e.g. a trained model); that dependency is **broadcast** — symlinked into every mapped invocation's `inputs/`. No annotation needed.

Nested maps (a map inside another map) are **not currently supported**. A map context must be closed by a reduce before another map begins.

---

## What `spade check` enforces in a collection

Running `spade check` (no arguments) in a collection directory walks every `blocks/*.yaml` and verifies:

1. All required fields are present (`id`, `version`, `inputs`, `outputs`).
2. Input/output `type` values are valid.
3. The `id` follows `<collection>.<block>`.
4. The entrypoint resolves to an existing file or subcommand for the language.
5. Map blocks output a `type: expansion`; reduce blocks accept a `type: collection` input.

Run it after creating or editing manifests. It catches the common mistakes before any pipeline runs.

---

## Worked example: a complete Python block

Manifest at `blocks/clip.yaml`:

```yaml
id: gdal.clip
version: 1.0.0
kind: standard
network: false
description: Clip a raster to a vector boundary, optionally with a buffer

inputs:
  source:
    type: file
    format: GeoTIFF
    description: Raster to clip
  boundary:
    type: file
    format: GeoJSON
    description: Vector boundary defining the clip region
  buffer_distance:
    type: number
    description: Buffer distance applied to the boundary, in CRS units

outputs:
  raster:
    type: file
    format: GeoTIFF
    description: Clipped raster output
```

Handler at `src/gdal_blocks/clip.py`:

```python
"""Block: clip a raster to a vector boundary."""
from spade import run, RasterFile, VectorFile
from osgeo import gdal


def handler(source: RasterFile, boundary: VectorFile, buffer_distance: float) -> RasterFile:
    out_path = "clipped.tif"
    gdal.Warp(
        out_path,
        source.path,
        cutlineDSName=boundary.path,
        cropToCutline=True,
        cutlineLayer=None,
    )
    return RasterFile(path=out_path)


if __name__ == "__main__":
    run(handler)
```

After saving both files, run `spade check` in the collection directory.

---

## Quick checklist

When asked to create a block:

1. **Detect the language.** Look at the root manifest file in the working directory.
2. **Pick the block name and ID.** Use `<collection>.<block>`.
3. **Write `blocks/<name>.yaml`.** Include `id`, `version`, `kind`, `network`, `description`, and full `inputs` / `outputs` with descriptions.
4. **Write the handler** in the language's standard source location, using the runtime library's types.
5. **Match manifest to handler.** Input/output names and types in the manifest must line up exactly with the handler's parameters and return value.
6. **Suggest `spade check`.**
