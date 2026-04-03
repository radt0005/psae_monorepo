+++
title = "Quickstart"
description = "Create your first Rust block step by step."
weight = 1
+++

This guide walks you through creating a Spade block in Rust. By the end you will have a working block that reads a raster file, applies a resolution parameter, and writes the result.

## Prerequisites

- **Rust stable** toolchain
- **Cargo** package manager
- The **Spade CLI** installed ([Installation guide](/getting-started/installation/))

## Step 1: Create a block collection

```bash
mkdir raster-tools && cd raster-tools
spade init --language rust
```

This scaffolds the project:

```
raster-tools/
  Cargo.toml
  src/
    lib.rs
  blocks/
```

Add the Spade library:

```bash
cargo add spade
```

## Step 2: Add a block

```bash
spade add reproject
```

This creates:

1. **`blocks/reproject.yaml`** -- the block manifest
2. **`src/reproject.rs`** -- the handler entrypoint

Register the module in `src/lib.rs` or create `src/main.rs` with the entry point.

## Step 3: Define the manifest

Edit `blocks/reproject.yaml`:

```yaml
id: raster-tools.reproject
version: 0.1.0
kind: standard
network: false
description: Reprojects a raster to a target resolution

inputs:
  source:
    type: file
    format: GeoTIFF
  resolution:
    type: number

outputs:
  raster:
    type: file
    format: GeoTIFF
```

## Step 4: Write the handler

Edit `src/main.rs`:

```rust
use spade::{run, Args, RasterFile};
use std::process::Command;

fn handler(args: Args) -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
    let source: RasterFile = args.input("source")?;
    let resolution: f64 = args.param("resolution")?;

    let output_path = "reprojected.tif";
    let status = Command::new("gdalwarp")
        .args([
            "-tr",
            &resolution.to_string(),
            &resolution.to_string(),
            &source.path,
            output_path,
        ])
        .status()?;

    if !status.success() {
        return Err("gdalwarp failed".into());
    }

    Ok(RasterFile::new(output_path))
}

fn main() {
    run(handler);
}
```

Key concepts:

- **`run(handler)`** is the main entry point. It loads inputs, parameters, calls your handler, and writes outputs.
- **`args.input::<T>(name)`** retrieves a typed file input. `T` must implement the `FromInput` trait.
- **`args.param::<T>(name)`** retrieves a typed scalar parameter from `params.yaml`. `T` must implement `serde::DeserializeOwned`.
- The handler returns `Result<O, Box<dyn Error + Send + Sync>>` where `O` implements `IntoOutput + SpadeType`.

## Step 5: Validate and install

```bash
spade check
spade install file://.
```

## Step 6: Use in a pipeline

```yaml
blocks:
  - id: 019cf4bc-a001-7000-0000-000000000000
    name: raster-tools.reproject
    inputs: []
    args:
      resolution: 10
```

## No-output handlers

If your block performs a side effect without producing output, return `()`:

```rust
fn handler(args: Args) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let source: RasterFile = args.input("source")?;
    println!("Validated: {}", source.path);
    Ok(())
}

fn main() {
    run(handler);
}
```

The `()` type implements `IntoOutput` and `SpadeType`, so `run()` skips the output-writing step.

## Next steps

- [Types](/libraries/rust/types/) -- all available Spade types
- [Handler Functions](/libraries/rust/handlers/) -- handler patterns and multiple outputs
- [Manifest Generation](/libraries/rust/manifest-generation/) -- generating manifests with the builder API
- [Examples](/libraries/rust/examples/) -- complete worked examples
