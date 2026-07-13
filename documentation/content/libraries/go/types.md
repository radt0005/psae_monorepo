+++
title = "Types"
description = "All available Spade types in the Go library."
weight = 2
+++

The Go library defines types as structs that implement a set of interfaces. Each type provides metadata for manifest generation, handles loading from the `inputs/` directory, and knows how to write itself to `outputs/`.

## Interfaces

### `SpadeType`

Provides metadata about a Spade type for manifest generation and output naming:

```go
type SpadeType interface {
    TypeName() string
    DefaultOutputName() string
    ManifestEntry() ManifestInfo
}
```

### `FromInput`

Constructs a typed value from raw filesystem input data:

```go
type FromInput interface {
    FromSingleFile(path string) error
    FromMultipleFiles(paths []string) error
    FromDirectory(path string) error
}
```

### `IntoOutput`

Writes a typed value to an output subdirectory:

```go
type IntoOutput interface {
    WriteTo(outputDir string) error
    DefaultOutputName() string
}
```

## Type reference

### Single-file types

| Type | `TypeName()` | Default output name | Format | Property |
|------|-------------|---------------------|--------|----------|
| `File` | `"file"` | `file` | -- | `Path string` |
| `RasterFile` | `"file"` | `raster` | `GeoTIFF` | `Path string` |
| `VectorFile` | `"file"` | `vector` | `GeoJSON` | `Path string` |
| `TabularFile` | `"file"` | `tabular` | `CSV` | `Path string` |
| `JsonFile` | `"json"` | `json` | -- | `Path string` |

Each has a constructor: `NewFile(path)`, `NewRasterFile(path)`, etc.

### Directory type

| Type | `TypeName()` | Default output name | Property |
|------|-------------|---------------------|----------|
| `Directory` | `"directory"` | `directory` | `Path string` |

Constructor: `NewDirectory(path)`.

### Collection types

| Type | `TypeName()` | Default output name | Format | Property |
|------|-------------|---------------------|--------|----------|
| `FileCollection` | `"collection"` | `files` | -- | `Paths []string` |
| `RasterFileCollection` | `"collection"` | `rasters` | `GeoTIFF` | `Paths []string` |
| `VectorFileCollection` | `"collection"` | `vectors` | `GeoJSON` | `Paths []string` |
| `TabularFileCollection` | `"collection"` | `tables` | `CSV` | `Paths []string` |

Each has a constructor: `NewFileCollection(paths)`, `NewRasterFileCollection(paths)`, etc.

### Scalar types (manifest-only)

These types are used only with the `ManifestBuilder` for declaring parameter types. They are not used at runtime:

| Type | `TypeName()` | Manifest `type` |
|------|-------------|-----------------|
| `StringType` | `"string"` | `string` |
| `NumberType` | `"number"` | `number` |
| `BoolType` | `"boolean"` | `boolean` |

## Using `Input` and `Param`

### File inputs with `Input[T]`

The generic `Input` function retrieves a typed input from `Args`. `T` must be a pointer to a struct implementing `FromInput`:

```go
// Single file
source, err := spade.Input[*spade.RasterFile](args, "source")
// source.Path contains the file path

// Collection
tiles, err := spade.Input[*spade.RasterFileCollection](args, "tiles")
// tiles.Paths contains a slice of file paths

// Directory
dir, err := spade.Input[*spade.Directory](args, "data")
// dir.Path contains the directory path
```

### Scalar parameters with `Param[T]`

The generic `Param` function retrieves a scalar parameter from `params.yaml`:

```go
resolution, err := spade.Param[float64](args, "resolution")
method, err := spade.Param[string](args, "method")
normalize, err := spade.Param[bool](args, "normalize")
count, err := spade.Param[int](args, "count")
```

The library handles type conversions between YAML's numeric types (`int`, `float64`, `int64`) automatically.

## Checking for optional inputs

Use `HasInput` and `HasParam` to check before accessing optional arguments:

```go
if args.HasInput("mask") {
    mask, err := spade.Input[*spade.RasterFile](args, "mask")
    // ...
}

if args.HasParam("buffer") {
    buffer, err := spade.Param[float64](args, "buffer")
    // ...
}
```

## Constructing output values

When returning from your handler, construct the appropriate type:

```go
// Single file
result := spade.NewRasterFile("result.tif")
return &result, nil

// Directory
result := spade.NewDirectory("output_dir")
return &result, nil

// Collection
result := spade.NewRasterFileCollection([]string{"tile_001.tif", "tile_002.tif"})
return &result, nil
```

## ManifestInfo

Each type's `ManifestEntry()` method returns a `ManifestInfo` struct used by the manifest builder:

```go
type ManifestInfo struct {
    TypeName string // "file", "directory", "collection", "json", "string", etc.
    Format   string // "GeoTIFF", "GeoJSON", "CSV", or empty
    ItemType string // "file" for collections, or empty
}
```

## Error types

The library defines typed errors for input/parameter access:

| Error type | When it occurs |
|------------|---------------|
| `ErrInputNotFound` | `Input()` called with a name not present in `inputs/` |
| `ErrParamNotFound` | `Param()` called with a name not present in `params.yaml` |
| `ErrEmptyInputDir` | An input subdirectory exists but contains no files |
| `ErrTypeMismatch` | Input data cannot be converted to the requested type |
| `ErrSecretNotFound` | `GetSecret()` called with a name not declared in the pipeline's `secrets:` map, or resolution failed |
