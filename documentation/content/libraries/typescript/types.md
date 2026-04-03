+++
title = "Types"
description = "All available Spade types in the TypeScript library."
weight = 2
+++

The TypeScript library provides classes for every Spade type. These classes are used as type annotations in handler metadata and as the values your handler receives and returns.

## Type reference

### Single-file types

| Class | Manifest `type` | Manifest `format` | Property | Description |
|-------|------------------|--------------------|----------|-------------|
| `File` | `file` | -- | `path: string` | A generic single file |
| `RasterFile` | `file` | `GeoTIFF` | `path: string` | A raster data file |
| `VectorFile` | `file` | `GeoJSON` | `path: string` | A vector data file |
| `TabularFile` | `file` | `CSV` | `path: string` | A tabular data file |
| `JsonFile` | `json` | -- | `path: string` | A JSON data file |

All single-file types extend `File` and expose a `path` property pointing to the file on disk.

### Directory type

| Class | Manifest `type` | Property | Description |
|-------|-----------------|----------|-------------|
| `Directory` | `directory` | `path: string` | A directory input or output |

### Collection types

| Class | Manifest `type` | Manifest `format` | Property | Description |
|-------|------------------|--------------------|----------|-------------|
| `FileCollection` | `collection` | -- | `paths: string[]` | A collection of generic files |
| `RasterFileCollection` | `collection` | `GeoTIFF` | `paths: string[]` | A collection of raster files |
| `VectorFileCollection` | `collection` | `GeoJSON` | `paths: string[]` | A collection of vector files |
| `TabularFileCollection` | `collection` | `CSV` | `paths: string[]` | A collection of tabular files |

Collection types extend `FileCollection` and expose a `paths` property with an array of file paths.

### Scalar types (parameters)

Scalar types are specified as string literals in metadata rather than as classes:

| Literal | Manifest `type` | TypeScript type | Description |
|---------|-----------------|-----------------|-------------|
| `"string"` | `string` | `string` | A text parameter |
| `"number"` | `number` | `number` | A numeric parameter |
| `"boolean"` | `boolean` | `boolean` | A boolean parameter |

## Class hierarchy

```
File
  RasterFile
  VectorFile
  TabularFile
  JsonFile

Directory

FileCollection
  RasterFileCollection
  VectorFileCollection
  TabularFileCollection
```

All single-file classes inherit from `File`. All collection classes inherit from `FileCollection`. `Directory` is a standalone class.

## Using types in metadata

Types appear in the `inputs` object of your `SpadeMetadata`. File and directory types use the class constructor; scalar types use string literals:

```typescript
import { spadeBlock, RasterFile, VectorFile } from "spade";

const handler = spadeBlock({
  inputs: {
    raster: RasterFile,       // file type -- uses class
    boundary: VectorFile,     // file type -- uses class
    buffer: "number",         // scalar type -- uses string literal
    label: "string",          // scalar type -- uses string literal
  },
  output: RasterFile,
})(function handler({ raster, boundary, buffer, label }) {
  // raster is a RasterFile instance
  // boundary is a VectorFile instance
  // buffer is a number
  // label is a string
  return new RasterFile("outputs/raster/result.tif");
});
```

## Constructing output values

When returning values from your handler, construct the appropriate type:

```typescript
// Single file
return new RasterFile("path/to/result.tif");

// Directory
return new Directory("path/to/output_dir");

// Collection
return new RasterFileCollection([
  "path/to/tile_001.tif",
  "path/to/tile_002.tif",
]);
```

## How input scanning works

When `run()` executes, it scans the `inputs/` directory. Each subdirectory becomes an input argument. The metadata's type hint controls how the input is constructed:

- **File subclass** -- the first file in the subdirectory is used to construct the instance
- **Directory** -- the subdirectory path itself is passed to the constructor
- **FileCollection subclass** -- all files in the subdirectory are collected into the array
- **Scalar type** -- the value comes from `params.yaml`, not from `inputs/`

If no metadata is provided, inputs default to `File` for single-file directories and `FileCollection` for multi-file directories.

## Importing

All types are exported from the main package:

```typescript
import {
  File,
  RasterFile,
  VectorFile,
  TabularFile,
  JsonFile,
  Directory,
  FileCollection,
  RasterFileCollection,
  VectorFileCollection,
  TabularFileCollection,
} from "spade";
```
