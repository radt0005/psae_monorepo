+++
title = "Handler Functions"
description = "Writing R handler functions with type annotations."
weight = 3
+++

A Spade block in R is an ordinary R function -- the **handler** -- plus metadata that tells the library how to wire it up. This page covers how to write handlers, attach type annotations, and handle single and multiple outputs.

## Anatomy of a handler

A handler is a plain R function whose parameter names match the block's input names and parameter names:

```r
handler <- function(source, buffer) {
  # processing logic
}
```

The library resolves each argument at runtime:

1. **File inputs** -- subdirectories under `inputs/` are matched by name and delivered as S4 objects (`File`, `RasterFile`, etc.).
2. **Scalar parameters** -- values from `params.yaml` are matched by name and delivered as plain R values (`numeric`, `character`, `logical`).

Inputs take precedence when a name appears in both `inputs/` and `params.yaml`.

## Secrets

Blocks that need credentials -- a database connection string, an API key -- request them by a logical name with `get_secret()`, rather than reading them from `params.yaml` or the process environment directly.

```r
library(spade)

handler <- function() {
  dsn <- get_secret("db")
  # ... connect with dsn and query ...
}

run(handler)
```

`get_secret(name)` returns the secret value (a length-one character string) bound to the logical `name`. The logical name is part of your block's contract, documented like any other parameter: the pipeline author binds it to one of their stored secrets via a `secrets:` map alongside `args:` in the pipeline YAML.

If `name` was not declared in the pipeline's `secrets:` map, or the bound secret failed to resolve, `get_secret()` calls `stop()`. A declared-but-unresolvable secret is a real error, not a silently empty string -- wrap the call in `tryCatch()` if you need to handle it, otherwise let it propagate like any other error (see Error handling below).

`get_secret()` never talks to a keychain or key-management service itself. It only reads values the worker or CLI already injected into the process environment before your script ran.

## Attaching type annotations

Use the `spade_types<-` replacement function to attach a named list of type annotations. Each key is a parameter name (or `.return` for the return type), and each value is a type name string:

```r
spade_types(handler) <- list(
  source = "RasterFile",
  buffer = "numeric",
  .return = "RasterFile"
)
```

Without type annotations, the library still works -- file inputs are delivered as generic `File` or `FileCollection` objects based on the number of files in each input directory. Type annotations let you request a specific subtype and enable manifest generation via `build()`.

## Attaching a description

Set the `spade_description` attribute to provide a human-readable description for the block manifest:

```r
attr(handler, "spade_description") <- "Buffer a raster by a given distance."
```

## The `run()` entry point

`run(handler)` is the single entry point that drives execution:

1. Loads scalar parameters from `params.yaml` via `yaml::read_yaml()`.
2. Scans `inputs/` subdirectories and constructs typed S4 objects based on your type annotations.
3. Merges the two sets of arguments into one named list (inputs override params on name collision).
4. Filters the argument list to only those names present in your handler's formal parameters (unless your handler accepts `...`).
5. Calls your handler via `do.call(fn, filtered_args)`.
6. Writes the handler's return value to `outputs/`.

Your script should end with:

```r
run(handler)
```

### Handlers with `...`

If your handler uses `...` instead of named parameters, the library passes all available arguments without filtering:

```r
handler <- function(...) {
  args <- list(...)
  # args$source, args$buffer, etc.
}
```

This is occasionally useful for generic or forwarding blocks, but named parameters are preferred because they make the block's interface explicit and enable manifest generation.

## Single output

Return a single S4 object to produce one output. The library writes it to `outputs/<name>/` where `<name>` is either inferred from the type or read from the block manifest:

```r
handler <- function(source, buffer) {
  r <- terra::rast(source@path)
  buffered <- terra::buffer(r, width = buffer)

  out_path <- file.path(tempdir(), "result.tif")
  terra::writeRaster(buffered, out_path, overwrite = TRUE)

  RasterFile(path = out_path)
}
spade_types(handler) <- list(
  source = "RasterFile",
  buffer = "numeric",
  .return = "RasterFile"
)
```

The inferred output directory for `RasterFile` is `outputs/raster/`. The full mapping is:

| Return type | Default output name |
|-------------|-------------------|
| `File` | `file` |
| `RasterFile` | `raster` |
| `VectorFile` | `vector` |
| `TabularFile` | `tabular` |
| `JsonFile` | `json` |
| `Directory` | `directory` |
| `FileCollection` | `files` |
| `RasterFileCollection` | `rasters` |
| `VectorFileCollection` | `vectors` |
| `TabularFileCollection` | `tables` |

If the block manifest declares exactly one output, the library uses that declared name instead of the default.

## Multiple outputs

Return a **named list** to produce multiple outputs. Each key becomes the output directory name:

```r
handler <- function(source) {
  r <- terra::rast(source@path)

  raster_path <- file.path(tempdir(), "processed.tif")
  terra::writeRaster(r, raster_path, overwrite = TRUE)

  stats_path <- file.path(tempdir(), "stats.json")
  jsonlite::write_json(list(mean = 42), stats_path)

  list(
    raster_out = RasterFile(path = raster_path),
    stats_out  = JsonFile(path = stats_path)
  )
}
```

This writes to `outputs/raster_out/` and `outputs/stats_out/`. The names in the returned list should match the output names declared in your block manifest.

## No output

If your handler returns `NULL` (or `invisible(NULL)`), the library writes nothing to `outputs/`. This is valid for blocks that produce side effects only, though most blocks return at least one output.

## Error handling

Errors raised inside your handler propagate normally. If your handler calls `stop()`, the block fails and the error message is reported by the Spade runtime:

```r
handler <- function(source) {
  if (!file.exists(source@path)) {
    stop("Input file does not exist: ", source@path)
  }
  # ...
}
```

There is no special error-handling API. Use standard R patterns (`tryCatch`, `stop`, `warning`) as needed.

## Complete template

Putting it all together, here is a minimal but complete block script:

```r
library(spade)

handler <- function(source, threshold) {
  # Your processing logic here
  r <- terra::rast(source@path)
  result <- terra::classify(r, cbind(-Inf, threshold, NA))

  out_path <- file.path(tempdir(), "classified.tif")
  terra::writeRaster(result, out_path, overwrite = TRUE)

  RasterFile(path = out_path)
}

spade_types(handler) <- list(
  source    = "RasterFile",
  threshold = "numeric",
  .return   = "RasterFile"
)
attr(handler, "spade_description") <- "Classify raster values below a threshold."

run(handler)
```
