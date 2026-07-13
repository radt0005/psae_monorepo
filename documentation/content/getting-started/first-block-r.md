+++
title = "Your First R Block"
description = "Create a custom Spade block in R from scratch."
weight = 4
+++

In this guide, you will create a custom block in R that reads a CSV file, groups the data by a column, and writes a summary as JSON. The entire block is a plain R function — no framework classes to inherit and no lifecycle hooks to implement.

This guide uses R. For other languages, see the guides for [Python](/getting-started/first-block/), [TypeScript](/libraries/typescript/quickstart/), [Go](/libraries/go/quickstart/), and [Rust](/libraries/rust/quickstart/).

## Prerequisites

Before starting, make sure you have:

- **R 4.0 or later** installed
- The **`spade`** R package: `install.packages("spade")`
- The **`jsonlite`** R package: `install.packages("jsonlite")`
- The **Spade CLI** installed ([Installation guide](/getting-started/installation/))

## Step 1: Create a block collection

A **collection** is a repository of related blocks that share a language. Create one:

```bash
mkdir field-stats && cd field-stats
spade init --language r
```

This scaffolds an R project:

```
field-stats/
  renv.lock       # renv dependency lockfile
  R/              # Handler scripts go here
  blocks/         # Block manifests go here
```

## Step 2: Add a block

```bash
spade add count-by-group
```

This creates two files:

1. **`blocks/count-by-group.yaml`** — The block manifest declaring inputs, outputs, and parameters
2. **`R/count_by_group.R`** — The handler function you will implement

## Step 3: Write the handler

Open `R/count_by_group.R` and replace its contents with:

```r
library(spade)
library(jsonlite)

handler <- function(data, group_by) {
  # Read the CSV via the @path slot
  df <- read.csv(data@path, stringsAsFactors = FALSE)

  if (!(group_by %in% names(df))) {
    stop("Column '", group_by, "' not found in data.")
  }

  # Count rows per group using base R
  counts <- as.list(table(df[[group_by]]))

  result <- list(
    total_rows   = nrow(df),
    group_column = group_by,
    n_groups     = length(counts),
    counts       = counts
  )

  out_path <- file.path(tempdir(), "summary.json")
  jsonlite::write_json(result, out_path, auto_unbox = TRUE, pretty = TRUE)

  JsonFile(path = out_path)
}

# Attach type annotations so the library knows how to load each argument
spade_types(handler) <- list(
  data     = "TabularFile",   # file input: delivered as a TabularFile S4 object
  group_by = "character",     # scalar: read from params.yaml
  .return  = "JsonFile"       # return type
)

attr(handler, "spade_description") <- "Count CSV records grouped by a column."

# Entry point: run() loads inputs, calls handler, writes outputs
run(handler)
```

Key points:

- **`data`** is declared as `"TabularFile"`, so the library delivers a `TabularFile` S4 object. Access the file path through its `@path` slot.
- **`group_by`** is declared as `"character"`, so the library reads it from `params.yaml` and passes it as a plain R character string.
- **`.return = "JsonFile"`** tells the library what kind of output the block produces.
- **`run(handler)`** is the entry point. It reads `params.yaml`, scans `inputs/`, calls your handler via `do.call()`, and writes outputs automatically. This one line is the only Spade-specific wiring.

## Step 4: Define the block manifest

Open `blocks/count-by-group.yaml` and replace its contents:

```yaml
id: field-stats.count-by-group
version: 0.1.0
kind: standard
network: false
description: Count CSV records grouped by a column.

inputs:
  data:
    type: file
    format: CSV
    description: The input CSV file to analyze
  group_by:
    type: string
    description: Name of the column to group by

outputs:
  json:
    type: json
    description: JSON file with row counts per group
```

This tells Spade that the block:

- Accepts a CSV file input named `data`
- Accepts a string parameter named `group_by`
- Produces a JSON output named `json`
- Does not need network access
- Is a standard (non-map, non-reduce) block

{% tip() %}
You can also generate the manifest from your type annotations instead of writing it by hand. After writing the handler, run this in R:

```r
library(spade)
source("R/count_by_group.R")
cat(yaml::as.yaml(build(handler)))
```

Copy the output into `blocks/count-by-group.yaml` and add the required `id`, `version`, `kind`, and `network` fields.
{% end %}

## Step 5: Validate

Check that the manifest and source file are consistent:

```bash
spade check
```

Expected output:

```
Collection 'field-stats' (r) is valid.
  1 block found: field-stats.count-by-group
```

`spade check` verifies that all required manifest fields are present, that the input and output types are valid, and that the handler file exists.

## Step 6: Install locally

Install the collection from the local directory:

```bash
spade install file://.
```

This registers the block in `~/.spade/blocks/field-stats/0.1.0/`. You can now reference `field-stats.count-by-group` in any pipeline.

## Step 7: Use in a pipeline

Create a pipeline that uses your new block. Create `test-pipeline.yaml`:

```yaml
name: field-stats-test
version: "1.0"
description: Test the count-by-group block

blocks:
  - id: "@count"
    name: field-stats.count-by-group
    inputs: []
    args:
      group_by: species
```

{% note() %}
For this test, you will need to provide an input CSV file. In a real pipeline, the input would come from an upstream block. For local testing, you can create a test working directory manually — see the [Testing Blocks](/tutorials/testing-blocks/) tutorial.
{% end %}

Run it:

```bash
spade run test-pipeline.yaml
```

## What you built

Here is the complete set of files you created:

**`R/count_by_group.R`** — the handler:

```r
library(spade)
library(jsonlite)

handler <- function(data, group_by) {
  df <- read.csv(data@path, stringsAsFactors = FALSE)
  if (!(group_by %in% names(df))) {
    stop("Column '", group_by, "' not found in data.")
  }
  counts <- as.list(table(df[[group_by]]))
  result <- list(
    total_rows   = nrow(df),
    group_column = group_by,
    n_groups     = length(counts),
    counts       = counts
  )
  out_path <- file.path(tempdir(), "summary.json")
  jsonlite::write_json(result, out_path, auto_unbox = TRUE, pretty = TRUE)
  JsonFile(path = out_path)
}

spade_types(handler) <- list(
  data     = "TabularFile",
  group_by = "character",
  .return  = "JsonFile"
)
attr(handler, "spade_description") <- "Count CSV records grouped by a column."

run(handler)
```

**`blocks/count-by-group.yaml`** — the manifest:

```yaml
id: field-stats.count-by-group
version: 0.1.0
kind: standard
network: false
description: Count CSV records grouped by a column.

inputs:
  data:
    type: file
    format: CSV
    description: The input CSV file to analyze
  group_by:
    type: string
    description: Name of the column to group by

outputs:
  json:
    type: json
    description: JSON file with row counts per group
```

## Next steps

- [Types (R)](/libraries/r/types/) — all available Spade types for inputs and outputs
- [Handler Functions (R)](/libraries/r/handlers/) — patterns for single and multiple outputs
- [Manifest Generation (R)](/libraries/r/manifest-generation/) — auto-generate manifests from type annotations
- [Examples (R)](/libraries/r/examples/) — complete worked examples including raster processing
- [Building a Block](/tutorials/building-a-block/) — a comprehensive end-to-end tutorial (Python, with notes for other languages)
