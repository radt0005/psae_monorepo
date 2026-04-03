+++
title = "Manifest Generation"
description = "Auto-generating block manifests from R type annotations."
weight = 4
+++

Every Spade block needs a manifest YAML file that declares its inputs, outputs, and metadata. Instead of writing this by hand, the R library can derive it from the type annotations you already attach to your handler.

## The `build()` function

`build(fn)` reads a handler's `spade_types` and `spade_description` attributes and returns a list representing the manifest:

```r
library(spade)

handler <- function(source, buffer, method) {
  NULL
}

spade_types(handler) <- list(
  source = "RasterFile",
  buffer = "numeric",
  method = "character",
  .return = "RasterFile"
)
attr(handler, "spade_description") <- "Buffer a raster by a given distance."

manifest <- build(handler)
```

The returned `manifest` list has this structure:

```r
list(
  description = "Buffer a raster by a given distance.",
  inputs = list(
    source = list(type = "file", format = "GeoTIFF"),
    buffer = list(type = "number"),
    method = list(type = "string")
  ),
  outputs = list(
    raster = list(type = "file", format = "GeoTIFF")
  )
)
```

## Writing the manifest to YAML

Convert the list to YAML with the `yaml` package:

```r
cat(yaml::as.yaml(build(handler)))
```

Output:

```yaml
description: Buffer a raster by a given distance.
inputs:
  source:
    type: file
    format: GeoTIFF
  buffer:
    type: number
  method:
    type: string
outputs:
  raster:
    type: file
    format: GeoTIFF
```

To write it directly to a file:

```r
yaml::write_yaml(build(handler), "blocks/buffer-raster.yaml")
```

You will typically add the `id`, `version`, `kind`, and `network` fields by hand (or via a script), since those are not derivable from the handler function alone.

## Type mapping reference

`build()` maps each type annotation string to a manifest entry according to this table:

| Type annotation | Manifest fields |
|-----------------|----------------|
| `"File"` | `type: file` |
| `"RasterFile"` | `type: file, format: GeoTIFF` |
| `"VectorFile"` | `type: file, format: GeoJSON` |
| `"TabularFile"` | `type: file, format: CSV` |
| `"JsonFile"` | `type: json` |
| `"Directory"` | `type: directory` |
| `"FileCollection"` | `type: collection, item_type: file` |
| `"RasterFileCollection"` | `type: collection, item_type: file, format: GeoTIFF` |
| `"VectorFileCollection"` | `type: collection, item_type: file, format: GeoJSON` |
| `"TabularFileCollection"` | `type: collection, item_type: file, format: CSV` |
| `"character"` | `type: string` |
| `"integer"` | `type: number` |
| `"numeric"` | `type: number` |
| `"logical"` | `type: boolean` |

## Output naming

When `build()` processes the `.return` type, it assigns a default output name based on the type:

| Return type | Output name |
|-------------|-------------|
| `RasterFile` | `raster` |
| `VectorFile` | `vector` |
| `TabularFile` | `tabular` |
| `JsonFile` | `json` |
| `File` | `file` |
| `Directory` | `directory` |
| `FileCollection` | `files` |
| `RasterFileCollection` | `rasters` |
| `VectorFileCollection` | `vectors` |
| `TabularFileCollection` | `tables` |

If the return type is not in this mapping, the output is named `"output"`.

## Parameters without type annotations

`build()` only includes parameters that have a corresponding entry in `spade_types`. If your handler accepts a parameter but you do not annotate it, that parameter is omitted from the generated manifest. This means you can incrementally annotate your handler -- unannotated parameters simply will not appear in the generated YAML.

## Handlers with no annotations

If no `spade_types` attribute is set, `build()` returns a manifest with empty `inputs` and `outputs`:

```r
handler <- function(x, y) NULL
build(handler)
#> $inputs
#> list()
#> $outputs
#> list()
```

## Workflow

A typical manifest-generation workflow:

1. Write your handler with full type annotations.
2. Run `build()` to generate the manifest list.
3. Merge in metadata fields (`id`, `version`, `kind`, `network`) that the handler cannot express.
4. Write the result to `blocks/<block-name>.yaml`.
5. Run `spade check` to validate consistency between the manifest and the handler.

```r
library(spade)
source("R/buffer_raster.R")

manifest <- build(handler)
manifest$id <- "raster-tools.buffer-raster"
manifest$version <- "0.1.0"
manifest$kind <- "standard"
manifest$network <- FALSE

yaml::write_yaml(manifest, "blocks/buffer-raster.yaml")
```

This keeps your manifest and implementation in sync. When you change the handler's signature or types, re-running `build()` updates the manifest automatically.
