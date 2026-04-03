+++
title = "Manifest Generation"
description = "Generating block manifests with the Go builder API."
weight = 4
+++

The Go library provides a `ManifestBuilder` with a fluent API for generating block manifest data programmatically. This keeps your manifest and implementation in sync by deriving type information directly from Go types.

## The `ManifestBuilder`

Create a builder, chain declarations, and call `Build()` to produce a `map[string]any` suitable for YAML serialization:

```go
import spade "github.com/spade-dev/spade"

manifest := spade.NewManifestBuilder().
    Description("Reprojects a raster to a target resolution").
    Build()
```

## Declaring inputs

Use the generic `ManifestInput[T]` function to declare an input. `T` must implement `SpadeType`:

```go
b := spade.NewManifestBuilder().
    Description("Clips a raster to a boundary")

spade.ManifestInput[spade.RasterFile](b, "source")
spade.ManifestInput[spade.VectorFile](b, "boundary")
spade.ManifestInput[spade.NumberType](b, "buffer")

manifest := b.Build()
```

Because Go generics do not support method-level type parameters on a receiver, `ManifestInput` and `ManifestOutput` are package-level functions that take the builder as their first argument. They return the builder for chaining:

```go
spade.ManifestInput[spade.RasterFile](b, "source")
// equivalent to: b = ManifestInput[RasterFile](b, "source")
```

## Declaring outputs

Use the generic `ManifestOutput[T]` function:

```go
spade.ManifestOutput[spade.RasterFile](b, "raster")
spade.ManifestOutput[spade.JsonFile](b, "stats")
```

## Complete example

```go
package main

import (
    "fmt"

    spade "github.com/spade-dev/spade"
    "gopkg.in/yaml.v3"
)

func main() {
    b := spade.NewManifestBuilder().
        Description("Reprojects a raster to a target resolution")

    spade.ManifestInput[spade.RasterFile](b, "source")
    spade.ManifestInput[spade.NumberType](b, "resolution")
    spade.ManifestOutput[spade.RasterFile](b, "raster")

    manifest := b.Build()

    data, _ := yaml.Marshal(manifest)
    fmt.Println(string(data))
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

Each Go type maps to a manifest entry through its `ManifestEntry()` method:

| Go type | Manifest `type` | `format` | `item_type` |
|---------|----------------|----------|-------------|
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
| `StringType` | `string` | -- | -- |
| `NumberType` | `number` | -- | -- |
| `BoolType` | `boolean` | -- | -- |

## The `ManifestInfo` struct

Each type's `ManifestEntry()` returns a `ManifestInfo`:

```go
type ManifestInfo struct {
    TypeName string // "file", "directory", "collection", "json", etc.
    Format   string // "GeoTIFF", "GeoJSON", "CSV", or empty
    ItemType string // "file" for collections, or empty
}
```

The builder converts this to a nested map: `{ "type": "file", "format": "GeoTIFF" }`. Empty fields are omitted.

## The `Build()` output

`Build()` returns a `map[string]any` with the following structure:

```go
map[string]any{
    "description": "...",
    "inputs": map[string]any{
        "source": map[string]any{
            "type": "file",
            "format": "GeoTIFF",
        },
    },
    "outputs": map[string]any{
        "raster": map[string]any{
            "type": "file",
            "format": "GeoTIFF",
        },
    },
}
```

Serialize this with `yaml.Marshal` or `json.Marshal` as needed.

## Mixed inputs example

Combine file inputs with scalar parameter types:

```go
b := spade.NewManifestBuilder().
    Description("Normalizes raster data")

spade.ManifestInput[spade.RasterFile](b, "raster")
spade.ManifestInput[spade.NumberType](b, "buffer")
spade.ManifestInput[spade.BoolType](b, "normalize")
spade.ManifestOutput[spade.RasterFile](b, "raster")

manifest := b.Build()
```

This generates inputs for a file, a number, and a boolean alongside a raster output.
