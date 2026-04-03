+++
title = "Examples"
description = "Complete worked examples of R blocks."
weight = 5
+++

Three complete examples demonstrating common block patterns: raster processing, tabular analysis, and a map block that processes items in parallel.

## Example 1: NDVI calculation from multispectral raster

This block reads a multispectral GeoTIFF, computes the Normalized Difference Vegetation Index (NDVI), and writes the result as a single-band raster.

### Handler

```r
library(spade)

handler <- function(source, red_band, nir_band) {
  r <- terra::rast(source@path)

  red <- r[[red_band]]
  nir <- r[[nir_band]]

  ndvi <- (nir - red) / (nir + red)

  out_path <- file.path(tempdir(), "ndvi.tif")
  terra::writeRaster(ndvi, out_path, overwrite = TRUE)

  RasterFile(path = out_path)
}

spade_types(handler) <- list(
  source   = "RasterFile",
  red_band = "integer",
  nir_band = "integer",
  .return  = "RasterFile"
)
attr(handler, "spade_description") <- "Compute NDVI from a multispectral raster."

run(handler)
```

### params.yaml

```yaml
red_band: 3
nir_band: 4
```

### Generated manifest

```r
cat(yaml::as.yaml(build(handler)))
```

```yaml
description: Compute NDVI from a multispectral raster.
inputs:
  source:
    type: file
    format: GeoTIFF
  red_band:
    type: number
  nir_band:
    type: number
outputs:
  raster:
    type: file
    format: GeoTIFF
```

### How it works

- `source` arrives as a `RasterFile`. The handler reads it with `terra::rast()` via the `@path` slot.
- `red_band` and `nir_band` are integers from `params.yaml`, used to select raster layers by index.
- The handler returns a single `RasterFile`, which the library writes to `outputs/raster/`.

---

## Example 2: CSV summary statistics

This block reads a CSV file, computes summary statistics for a specified column, and writes the results as JSON.

### Handler

```r
library(spade)

handler <- function(data, column, na_rm) {
  df <- read.csv(data@path)

  if (!(column %in% names(df))) {
    stop("Column '", column, "' not found in input CSV.")
  }

  values <- df[[column]]
  if (!is.numeric(values)) {
    values <- suppressWarnings(as.numeric(values))
  }

  stats <- list(
    column = column,
    n      = length(values),
    n_valid = sum(!is.na(values)),
    mean   = mean(values, na.rm = na_rm),
    sd     = sd(values, na.rm = na_rm),
    min    = min(values, na.rm = na_rm),
    max    = max(values, na.rm = na_rm),
    q25    = unname(quantile(values, 0.25, na.rm = na_rm)),
    q50    = unname(quantile(values, 0.50, na.rm = na_rm)),
    q75    = unname(quantile(values, 0.75, na.rm = na_rm))
  )

  out_path <- file.path(tempdir(), "summary.json")
  jsonlite::write_json(stats, out_path, auto_unbox = TRUE, pretty = TRUE)

  JsonFile(path = out_path)
}

spade_types(handler) <- list(
  data   = "TabularFile",
  column = "character",
  na_rm  = "logical",
  .return = "JsonFile"
)
attr(handler, "spade_description") <- "Compute summary statistics for a CSV column."

run(handler)
```

### params.yaml

```yaml
column: temperature
na_rm: true
```

### Generated manifest

```yaml
description: Compute summary statistics for a CSV column.
inputs:
  data:
    type: file
    format: CSV
  column:
    type: string
  na_rm:
    type: boolean
outputs:
  json:
    type: json
```

### How it works

- `data` arrives as a `TabularFile`. The handler reads it with `read.csv()`.
- `column` and `na_rm` are scalars from `params.yaml`.
- The handler computes statistics using base R functions and writes the result as JSON.
- Returning a `JsonFile` causes the library to write it to `outputs/json/`.

---

## Example 3: Tile merger (map block)

This block demonstrates working with a collection of raster tiles. It receives multiple GeoTIFF files, merges them into a single raster, and also produces a JSON metadata sidecar.

### Handler

```r
library(spade)

handler <- function(tiles, nodata) {
  # Load all tiles from the collection
  rasters <- lapply(tiles@paths, terra::rast)

  # Merge into a single raster
  if (length(rasters) == 1) {
    merged <- rasters[[1]]
  } else {
    merged <- do.call(terra::merge, rasters)
  }

  # Replace nodata sentinel
  merged[merged == nodata] <- NA

  # Write the merged raster
  raster_path <- file.path(tempdir(), "merged.tif")
  terra::writeRaster(merged, raster_path, overwrite = TRUE)

  # Write metadata
  meta <- list(
    n_tiles   = length(tiles@paths),
    crs       = terra::crs(merged, describe = TRUE)$code,
    extent    = as.list(as.vector(terra::ext(merged))),
    n_cells   = terra::ncell(merged),
    res       = as.list(terra::res(merged))
  )
  meta_path <- file.path(tempdir(), "metadata.json")
  jsonlite::write_json(meta, meta_path, auto_unbox = TRUE, pretty = TRUE)

  # Return multiple outputs as a named list
  list(
    merged   = RasterFile(path = raster_path),
    metadata = JsonFile(path = meta_path)
  )
}

spade_types(handler) <- list(
  tiles  = "RasterFileCollection",
  nodata = "numeric"
)
attr(handler, "spade_description") <- "Merge raster tiles and produce metadata."

run(handler)
```

### params.yaml

```yaml
nodata: -9999
```

### Block manifest

```yaml
id: raster-tools.merge-tiles
version: 0.1.0
kind: standard
network: false
description: Merge raster tiles and produce metadata.

inputs:
  tiles:
    type: collection
    item_type: file
    format: GeoTIFF
  nodata:
    type: number

outputs:
  merged:
    type: file
    format: GeoTIFF
  metadata:
    type: json
```

### How it works

- `tiles` arrives as a `RasterFileCollection`. The `@paths` slot is a character vector, so `lapply()` iterates over each file path to load them with `terra::rast()`.
- `nodata` is a numeric scalar from `params.yaml`.
- The handler returns a **named list** with two entries, producing two outputs: the merged raster at `outputs/merged/` and the metadata JSON at `outputs/metadata/`.
- The keys of the returned list (`merged`, `metadata`) correspond to the output names declared in the block manifest.

## Patterns to note

Across all three examples, the structure is consistent:

1. **Load inputs** by reading from the `@path` or `@paths` slot of the S4 objects.
2. **Read scalars** directly -- they arrive as plain R values.
3. **Process** using any R package (`terra`, `jsonlite`, base R, etc.).
4. **Write results** to temporary files, then wrap them in the appropriate Spade type constructor.
5. **Return** a single typed object or a named list for multiple outputs.
6. **`run(handler)`** at the end handles everything else.
