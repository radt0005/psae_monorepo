+++
title = "Your First Pipeline"
description = "Create, validate, and run a simple two-block pipeline."
weight = 2
+++

A **pipeline** is a series of processing steps connected together. Each step is a **block** — a self-contained unit of computation. In this guide, you will create a simple pipeline that downloads satellite imagery and reprojects it to a new coordinate system.

## What you'll build

This pipeline uses two blocks:

1. **`data.sentinel2`** — Downloads Sentinel-2 satellite imagery for a region
2. **`raster.reproject`** — Reprojects the downloaded raster to a different coordinate system

Data flows from the first block's output into the second block's input.

## Prerequisites

Make sure you have:

- The Spade CLI installed ([Installation guide](/getting-started/installation/))
- The core and GDAL block collections installed:

```bash
spade install https://github.com/spade-dev/core-blocks.git
spade install https://github.com/spade-dev/gdal-blocks.git
```

## Write the pipeline YAML

Create a file called `reproject-pipeline.yaml`:

```yaml
name: reproject-example
version: "1.0"
description: Download satellite imagery and reproject it

blocks:
  - id: "@source"
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"
      date_range: "2025-01-01/2025-06-01"

  - id: "@reproject"
    name: raster.reproject
    inputs:
      - "@source"
    args:
      target_crs: "EPSG:4326"
```

Block IDs use `@source` and `@reproject` — **short codes** — rather than long UUID strings. Short codes are the recommended form for hand-authored pipelines: they are readable, diff-friendly, and easy to type correctly. The CLI resolves them to stable UUIDs automatically on the first `spade check` or `spade run` and stores the bindings in a sibling `reproject-pipeline.lock.yaml` file.

Let's walk through each field:

- **`name`** — A human-readable name for the pipeline
- **`version`** — The pipeline version (must be a quoted string)
- **`blocks`** — The list of processing steps

For each block:

- **`id`** — A short code (`@<identifier>`) uniquely identifying this block invocation within the pipeline. Short codes must start with a letter or underscore and contain only letters, digits, and underscores after the `@`.
- **`name`** — Which block to run (format: `collection.block`)
- **`inputs`** — Which earlier blocks provide data to this one. An empty list `[]` means this block has no dependencies (it is a source block that runs first).
- **`args`** — Parameters passed to the block at runtime

The second block lists `"@source"` in its `inputs`. This tells Spade that the second block depends on the first block's output. Spade automatically matches the output type of `data.sentinel2` (a raster file) to the input type expected by `raster.reproject`.

{% note() %}
The pipeline-level `id` is omitted here — the CLI generates a fresh UUID at run time. This is the recommended pattern for hand-authored pipelines. See [Short Codes and Hand-Authoring](/pipelines/short-codes/) for the full reference.
{% end %}

## Validate the pipeline

Before running, check that the pipeline is valid:

```bash
spade check reproject-pipeline.yaml
```

If everything is correct, you'll see:

```
Pipeline 'reproject-example' is valid.
  2 blocks, 0 errors.
```

The first time you run `spade check` (or `spade run`), the CLI also creates a `reproject-pipeline.lock.yaml` file alongside your pipeline:

```yaml
# reproject-pipeline.lock.yaml
pipeline: reproject-example
version: "1.0"
bindings:
  "@source":    019cf4bc-1111-7000-0000-000000000001
  "@reproject": 019cf4bc-2222-7000-0000-000000000002
```

This file stores the UUID assigned to each short code so that reruns use the same IDs — enabling Spade's result cache to work correctly.

{% tip() %}
Commit `reproject-pipeline.lock.yaml` to version control alongside the pipeline file, the same way you would commit a `package-lock.json` or `Cargo.lock`. This lets collaborators reproduce your cache hits.
{% end %}

If there's an issue — for example, a missing block or an invalid reference — `spade check` will describe the problem precisely.

## Run the pipeline

Execute the pipeline locally:

```bash
spade run reproject-pipeline.yaml
```

Spade will:

1. Resolve block dependencies
2. Execute `data.sentinel2` first (since it has no inputs)
3. Pass its output to `raster.reproject`
4. Execute `raster.reproject`
5. Report success

You should see output like:

```
Running pipeline 'reproject-example'...
  [1/2] data.sentinel2 .......... done (3.2s)
  [2/2] raster.reproject ........ done (1.1s)
Pipeline complete! (4.3s total)
```

The `--no-ui` flag gives you simpler line-by-line output if you prefer:

```bash
spade run --no-ui reproject-pipeline.yaml
```

## Inspect the results

Pipeline working directories are stored in `~/.spade/pipelines/`. To keep the working directory after the pipeline finishes (it's normally cleaned up), use:

```bash
spade run --keep-work-dir reproject-pipeline.yaml
```

Inside the working directory, each block invocation has its own folder with `inputs/`, `outputs/`, and `logs/` subdirectories.

## Next steps

Now that you've run a pipeline, learn how to create your own block:

- [Your First Block (Python)](/getting-started/first-block/)
- [Your First Block (R)](/getting-started/first-block-r/)
- Or go straight to the [library documentation](/libraries/) for your language.
