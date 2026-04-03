+++
title = "Quickstart"
description = "Create your first R block step by step."
weight = 1
+++

This guide walks you through creating, annotating, and running an R block from scratch. By the end you will have a working block that reads a raster file, applies a buffer, and writes the result.

## Prerequisites

- R 4.0 or later
- The `spade` R package installed (`install.packages("spade")`)
- The Spade CLI installed ([Installation guide](/getting-started/installation/))

## Step 1: Scaffold a block collection

Create a new collection and initialize it for R:

```bash
mkdir raster-tools && cd raster-tools
spade init --language r
```

This produces:

```
raster-tools/
  DESCRIPTION
  R/
  blocks/
```

## Step 2: Add a block

```bash
spade add buffer-raster
```

Spade creates two files:

1. **`blocks/buffer-raster.yaml`** -- the block manifest
2. **`R/buffer_raster.R`** -- the handler you will implement

## Step 3: Write the handler

Open `R/buffer_raster.R` and replace its contents:

```r
library(spade)

handler <- function(source, buffer) {
  # Read the input raster via its @path slot
  r <- terra::rast(source@path)

  # Apply the buffer (simplified example)
  buffered <- terra::buffer(r, width = buffer)

  # Write the result
  out_path <- file.path(tempdir(), "buffered.tif")
  terra::writeRaster(buffered, out_path, overwrite = TRUE)

  RasterFile(path = out_path)
}

# Attach type annotations
spade_types(handler) <- list(
  source = "RasterFile",
  buffer = "numeric",
  .return = "RasterFile"
)

# Attach a description
attr(handler, "spade_description") <- "Buffer a raster by a given distance."

# Entry point
run(handler)
```

Key points:

- **`source`** is declared as `"RasterFile"`, so the library delivers a `RasterFile` S4 object whose `@path` slot points to the actual file.
- **`buffer`** is declared as `"numeric"`, so the library reads it from `params.yaml` and passes it as a plain R numeric value.
- **`.return`** tells the library (and the manifest builder) that the block produces a raster output.
- **`run(handler)`** is the entry point. It loads `params.yaml`, scans `inputs/`, calls your handler via `do.call()`, and writes outputs automatically.

## Step 4: Generate the manifest

Instead of writing the manifest by hand, let the library derive it from your type annotations:

```r
library(spade)
source("R/buffer_raster.R")
cat(yaml::as.yaml(build(handler)))
```

This prints:

```yaml
description: Buffer a raster by a given distance.
inputs:
  source:
    type: file
    format: GeoTIFF
  buffer:
    type: number
outputs:
  raster:
    type: file
    format: GeoTIFF
```

Copy this into `blocks/buffer-raster.yaml` (adding the required `id`, `version`, `kind`, and `network` fields), or pipe it directly.

## Step 5: Validate and install

```bash
spade check
spade install file://.
```

`spade check` confirms the manifest and handler are consistent. `spade install` registers the block locally so pipelines can reference it.

## Next steps

- [Types](/libraries/r/types/) -- all available Spade types and when to use each one
- [Handler Functions](/libraries/r/handlers/) -- patterns for single and multiple outputs
- [Manifest Generation](/libraries/r/manifest-generation/) -- the full `build()` workflow
- [Examples](/libraries/r/examples/) -- complete worked examples with real R packages
