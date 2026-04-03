+++
title = "Types"
description = "All available Spade types in the R library."
weight = 2
+++

The R Spade library uses **S4 classes** to represent typed inputs and outputs. If you have not encountered S4 before: S4 is R's formal object-oriented system, defined with `setClass()`. Each S4 object has named **slots** (similar to fields or attributes in other languages) that you access with the `@` operator instead of `$`. The Spade library defines these classes for you -- you only need to know how to read their slots and construct them for return values.

## File types

All file types inherit from the base `File` class and carry a single slot, `@path`, containing the absolute path to the file on disk.

| Class | Description | Manifest mapping |
|-------|-------------|------------------|
| `File` | Generic file of any format | `type: file` |
| `RasterFile` | Raster data (GeoTIFF, etc.) | `type: file, format: GeoTIFF` |
| `VectorFile` | Vector data (GeoJSON, etc.) | `type: file, format: GeoJSON` |
| `TabularFile` | Tabular data (CSV, etc.) | `type: file, format: CSV` |
| `JsonFile` | JSON file | `type: json` |

### Using file types

When the library delivers a file input to your handler, you access the path through the `@path` slot:

```r
handler <- function(source) {
  # Read the file using any R package
  r <- terra::rast(source@path)
  # ...
}
spade_types(handler) <- list(source = "RasterFile")
```

When returning a file output, construct the appropriate type with its constructor function:

```r
out_path <- file.path(tempdir(), "result.tif")
terra::writeRaster(result, out_path, overwrite = TRUE)
RasterFile(path = out_path)
```

Each constructor (`File()`, `RasterFile()`, `VectorFile()`, `TabularFile()`, `JsonFile()`) takes a single `path` argument.

## Directory type

The `Directory` class represents a directory input or output. Like file types, it carries a `@path` slot pointing to the directory.

| Class | Description | Manifest mapping |
|-------|-------------|------------------|
| `Directory` | A directory of files | `type: directory` |

```r
handler <- function(tile_dir) {
  tifs <- list.files(tile_dir@path, pattern = "\\.tif$", full.names = TRUE)
  # process each file ...
}
spade_types(handler) <- list(tile_dir = "Directory")
```

## Collection types

Collection types represent multiple files. They carry a `@paths` slot -- a character vector of file paths.

| Class | Description | Manifest mapping |
|-------|-------------|------------------|
| `FileCollection` | Generic file collection | `type: collection, item_type: file` |
| `RasterFileCollection` | Collection of raster files | `type: collection, item_type: file, format: GeoTIFF` |
| `VectorFileCollection` | Collection of vector files | `type: collection, item_type: file, format: GeoJSON` |
| `TabularFileCollection` | Collection of tabular files | `type: collection, item_type: file, format: CSV` |

### Using collection types

The `@paths` slot is a standard character vector, so you can iterate over it with `lapply()`, `for`, or any other R idiom:

```r
handler <- function(tiles) {
  rasters <- lapply(tiles@paths, terra::rast)
  merged <- do.call(terra::merge, rasters)
  # ...
}
spade_types(handler) <- list(tiles = "RasterFileCollection")
```

To return a collection, pass the character vector of output paths to the constructor:

```r
out_paths <- c("outputs/a.tif", "outputs/b.tif")
RasterFileCollection(paths = out_paths)
```

## Scalar types

Scalar parameters come from `params.yaml` and map to ordinary R types. You do not need to construct S4 objects for scalars -- the library passes them as plain R values.

| Type annotation | R type | Manifest mapping |
|-----------------|--------|------------------|
| `"numeric"` | `numeric` | `type: number` |
| `"integer"` | `integer` | `type: number` |
| `"character"` | `character` | `type: string` |
| `"logical"` | `logical` | `type: boolean` |

```r
handler <- function(source, buffer, method, verbose) {
  if (verbose) message("Using method: ", method)
  # buffer is a plain numeric, method is a character string
  # ...
}
spade_types(handler) <- list(
  source   = "RasterFile",
  buffer   = "numeric",
  method   = "character",
  verbose  = "logical",
  .return  = "RasterFile"
)
```

## Inheritance hierarchy

The S4 class hierarchy is straightforward:

```
File
 ├── RasterFile
 ├── VectorFile
 ├── TabularFile
 └── JsonFile

Directory

FileCollection
 ├── RasterFileCollection
 ├── VectorFileCollection
 └── TabularFileCollection
```

All file subtypes inherit from `File`, so any function that accepts a `File` also works with `RasterFile`, `VectorFile`, and so on. The same holds for collection subtypes and `FileCollection`.

## Validation

The library validates objects at construction time. A `File` or `Directory` must have a non-empty, single-element character string for `@path`. A `FileCollection` must have a character vector for `@paths`. Invalid objects raise an error immediately:

```r
File(path = "")
#> Error: path must be a non-empty single character string

File(path = c("a.tif", "b.tif"))
#> Error: path must be a non-empty single character string
```
