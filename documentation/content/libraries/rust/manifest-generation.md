+++
title = "Manifest Generation"
description = "Generating block manifests with the Rust builder API."
weight = 4
+++

The Rust library provides a `ManifestBuilder` with a fluent API for generating block manifest data programmatically. This keeps your manifest and implementation in sync by deriving type information directly from Rust types.

## The `ManifestBuilder`

Create a builder, chain declarations, and call `build()` to produce a `HashMap<String, serde_yaml::Value>` suitable for YAML serialization:

```rust
use spade::{build, RasterFile};

let manifest = build()
    .description("Reprojects a raster to a target resolution")
    .input::<RasterFile>("source")
    .input::<f64>("resolution")
    .output::<RasterFile>("raster")
    .build();
```

## The `build()` convenience function

The `build()` function at the crate root creates a new `ManifestBuilder`:

```rust
use spade::build;

let manifest = build()
    .input::<RasterFile>("source")
    .output::<RasterFile>("raster")
    .build();
```

This is equivalent to `ManifestBuilder::new()`.

## Declaring inputs

Use the `input::<T>(name)` method. `T` must implement `SpadeType`:

```rust
use spade::{build, RasterFile, VectorFile};

let manifest = build()
    .input::<RasterFile>("source")
    .input::<VectorFile>("boundary")
    .input::<f64>("buffer")
    .input::<String>("method")
    .input::<bool>("normalize")
    .build();
```

Each input declaration reads the type's `manifest_entry()` to produce the correct manifest structure.

## Declaring outputs

Use the `output::<T>(name)` method:

```rust
use spade::{build, RasterFile, JsonFile};

let manifest = build()
    .input::<RasterFile>("source")
    .output::<RasterFile>("raster")
    .output::<JsonFile>("stats")
    .build();
```

## Setting a description

Use the `description(desc)` method:

```rust
let manifest = build()
    .description("Clips a raster to a vector boundary")
    .input::<RasterFile>("source")
    .output::<RasterFile>("raster")
    .build();
```

If no description is set, the key is omitted from the output.

## Complete example

```rust
use spade::{build, RasterFile, VectorFile, JsonFile, RasterFileCollection};

fn main() {
    let manifest = build()
        .description("Reprojects a raster to a target resolution")
        .input::<RasterFile>("source")
        .input::<f64>("resolution")
        .output::<RasterFile>("raster")
        .build();

    let yaml = serde_yaml::to_string(&manifest).unwrap();
    println!("{}", yaml);
}
```

Output:

```yaml
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

## Type-to-manifest mapping

Each Rust type maps to a manifest entry through its `SpadeType::manifest_entry()` implementation:

| Rust type | Manifest `type` | `format` | `item_type` |
|-----------|----------------|----------|-------------|
| `File` | `file` | -- | -- |
| `RasterFile` | `file` | `GeoTIFF` | -- |
| `VectorFile` | `file` | `GeoJSON` | -- |
| `TabularFile` | `file` | `CSV` | -- |
| `JsonFile` | `json` | -- | -- |
| `Directory` | `directory` | -- | -- |
| `FileCollection` | `collection` | -- | `file` |
| `RasterFileCollection` | `collection` | `GeoTIFF` | `file` |
| `VectorFileCollection` | `collection` | `GeoJSON` | `file` |
| `TabularFileCollection` | `collection` | `CSV` | `file` |
| `String` | `string` | -- | -- |
| `f64`, `f32`, `i64`, `i32` | `number` | -- | -- |
| `bool` | `boolean` | -- | -- |

## The `ManifestEntry` struct

Internally, the builder stores each input/output as a `ManifestEntry`:

```rust
pub struct ManifestEntry {
    pub type_name: String,
    pub format: Option<String>,
    pub item_type: Option<String>,
}
```

This is converted to a YAML mapping with `type`, `format` (if present), and `item_type` (if present).

## The `build()` output

The `build()` method returns `HashMap<String, serde_yaml::Value>`:

- `"description"` -- a `Value::String` (only if set)
- `"inputs"` -- a `Value::Mapping` of input name to `{type, format?, item_type?}`
- `"outputs"` -- a `Value::Mapping` of output name to `{type, format?, item_type?}`

Serialize with `serde_yaml::to_string()`:

```rust
let yaml_str = serde_yaml::to_string(&manifest)?;
```

## Mixed inputs example

Combine file inputs with scalar parameter types:

```rust
let manifest = build()
    .description("Normalizes raster data")
    .input::<RasterFile>("raster")
    .input::<i64>("buffer")
    .input::<bool>("normalize")
    .output::<RasterFile>("raster")
    .build();
```

This generates manifest entries for a file input, a numeric parameter, a boolean parameter, and a raster output.

## Using `ManifestBuilder::new()` directly

You can use `ManifestBuilder::new()` instead of the `build()` convenience function:

```rust
use spade::ManifestBuilder;

let manifest = ManifestBuilder::new()
    .description("Processes input data")
    .input::<RasterFile>("source")
    .output::<RasterFile>("raster")
    .build();
```

Both approaches are equivalent.
