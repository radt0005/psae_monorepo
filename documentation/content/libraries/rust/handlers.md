+++
title = "Handler Functions"
description = "Writing Rust handlers with traits and closures."
weight = 3
+++

A handler is a function or closure that receives an `Args` struct and returns a `Result` with a typed output. The Spade library calls your handler after loading all inputs and parameters.

## Basic handler pattern

Every handler follows this signature:

```rust
fn handler(args: Args) -> Result<O, Box<dyn Error + Send + Sync>>
```

Where `O` implements both `IntoOutput` and `SpadeType`. Pass it to `run()`:

```rust
use spade::{run, Args, RasterFile};

fn handler(args: Args) -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
    let source: RasterFile = args.input("source")?;
    let resolution: f64 = args.param("resolution")?;

    // Process the raster...
    Ok(RasterFile::new("result.tif"))
}

fn main() {
    run(handler);
}
```

## The `run` function

```rust
pub fn run<F, O>(handler: F)
where
    F: FnOnce(Args) -> Result<O, Box<dyn Error + Send + Sync>>,
    O: IntoOutput + SpadeType + 'static,
```

`run` is the main entry point. It:

1. **Loads parameters** from `params.yaml`
2. **Scans inputs** from the `inputs/` directory
3. **Builds the `Args` struct** merging parameters and inputs
4. **Calls your handler** with the `Args`
5. **Writes outputs** to the `outputs/` directory
6. **Exits** with code 1 on any error, printing the message to stderr with a `spade:` prefix

The handler can be a function pointer or a closure.

## Using closures

Closures work as handlers, which is useful for capturing values:

```rust
use spade::{run, Args, RasterFile};

fn main() {
    run(|args: Args| -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
        let source: RasterFile = args.input("source")?;
        Ok(RasterFile::new("result.tif"))
    });
}
```

## Accessing inputs

Use `args.input::<T>(name)` to retrieve typed file inputs. `T` must implement `FromInput`:

```rust
// Single file
let source: RasterFile = args.input("source")?;
// source.path is a String with the file path

// Collection
let tiles: RasterFileCollection = args.input("tiles")?;
// tiles.paths is a Vec<String>

// Directory
let data: Directory = args.input("data")?;
// data.path is a String with the directory path
```

The library scans `inputs/<name>/` and calls the appropriate `FromInput` method based on whether a single file or multiple files were found.

## Accessing parameters

Use `args.param::<T>(name)` to retrieve scalar parameters. `T` must implement `serde::de::DeserializeOwned`:

```rust
let resolution: f64 = args.param("resolution")?;
let method: String = args.param("method")?;
let normalize: bool = args.param("normalize")?;
```

## Single output

When the handler returns a single typed value, the library writes it to `outputs/`. The output directory name is determined by:

1. The block manifest, if it declares exactly one output
2. Otherwise, the type's `default_output_name()` (e.g., `"raster"` for `RasterFile`)

```rust
fn handler(args: Args) -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
    let source: RasterFile = args.input("source")?;
    Ok(RasterFile::new("result.tif"))
}
// Written to outputs/raster/result.tif (or the manifest-declared name)
```

## Multiple outputs

Use the `Outputs` collection to return multiple named outputs:

```rust
use spade::{run, Args, Outputs, RasterFile, JsonFile};

fn handler(args: Args) -> Result<Outputs, Box<dyn std::error::Error + Send + Sync>> {
    let source: RasterFile = args.input("source")?;

    // Process and create output files...
    let mut outputs = Outputs::new();
    outputs.add("raster", RasterFile::new("processed.tif"));
    outputs.add("stats", JsonFile::new("stats.json"));

    Ok(outputs)
}

fn main() {
    run(handler);
}
```

This writes:

```
outputs/
  raster/
    processed.tif
  stats/
    stats.json
```

### `Outputs::single`

For a single output using the type's default name:

```rust
let outputs = Outputs::single(RasterFile::new("result.tif"));
Ok(outputs)
// Written to outputs/raster/result.tif
```

## No-output handlers

If your block performs a side effect without producing files, return `()`:

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

## Optional inputs and parameters

Use `has_input` and `has_param` to check for optional arguments:

```rust
fn handler(args: Args) -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
    let source: RasterFile = args.input("source")?;

    // Optional mask input
    let mask: Option<VectorFile> = if args.has_input("mask") {
        Some(args.input("mask")?)
    } else {
        None
    };

    // Optional buffer parameter with default
    let buffer: f64 = if args.has_param("buffer") {
        args.param("buffer")?
    } else {
        0.0
    };

    // Process with source, mask, buffer...
    Ok(RasterFile::new("result.tif"))
}
```

## Error handling

The handler returns `Result<O, Box<dyn Error + Send + Sync>>`. Use the `?` operator to propagate errors from library calls, and construct custom errors as needed:

```rust
fn handler(args: Args) -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
    let source: RasterFile = args.input("source")?;

    if !source.path.ends_with(".tif") {
        return Err(format!("expected GeoTIFF, got {}", source.path).into());
    }

    // Processing...
    Ok(RasterFile::new("result.tif"))
}
```

The `SpadeError` enum also provides structured error variants:

```rust
use spade::SpadeError;

// These are returned by args.input() and args.param() automatically:
// SpadeError::InputNotFound { name }
// SpadeError::ParamNotFound { name }
// SpadeError::EmptyInputDir { name }
// SpadeError::TypeMismatch { name, expected, found }
// SpadeError::IoError(io::Error)
// SpadeError::YamlError(serde_yaml::Error)
```

All `SpadeError` variants implement `std::error::Error` via `thiserror`, so they work with `?` and `Box<dyn Error>`.
