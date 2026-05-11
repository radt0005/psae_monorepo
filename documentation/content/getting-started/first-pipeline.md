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
id: 019cf4bc-0000-7000-0000-000000000000
name: reproject-example
version: "1.0"
description: Download satellite imagery and reproject it

blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"
      date_range: "2025-01-01/2025-06-01"

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.reproject
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
    args:
      target_crs: "EPSG:4326"
```

Let's walk through each field:

- **`id`** — A unique identifier for the pipeline (UUIDv7 format)
- **`name`** — A human-readable name
- **`version`** — The pipeline version
- **`blocks`** — The list of processing steps

For each block:

- **`id`** — A unique invocation ID (UUIDv7 format, unique within this pipeline)
- **`name`** — Which block to run (format: `collection.block`)
- **`inputs`** — Which earlier blocks provide data to this one. An empty list `[]` means this block has no dependencies (it's a source block)
- **`args`** — Parameters passed to the block at runtime

Notice that the second block lists the first block's ID in its `inputs`. This tells Spade that the second block depends on the first block's output. Spade automatically matches the output type of `data.sentinel2` (a raster file) to the input type expected by `raster.reproject`.

### Easier authoring with short codes

Typing UUIDv7 strings is tedious. For hand-authored pipelines, Spade accepts `@`-prefixed **short codes** as a friendlier alternative. The same pipeline using short codes looks like this:

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

Notice that the pipeline-level `id` is omitted -- the CLI generates one at run time. The CLI resolves the short codes (`@source`, `@reproject`) into UUIDv7s on the first `spade check` or `spade run`, writing the bindings to a sibling `pipeline.lock.yaml` so the same labels resolve to the same UUIDs on subsequent runs.

The rest of this tutorial uses the UUID form for clarity, but everything works identically with short codes. See [Short Codes and Hand-Authoring](/pipelines/short-codes/) for the full reference.

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

If there's an issue — for example, a missing block or an invalid reference — Spade will describe the problem.

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

Now that you've run a pipeline, learn how to [create your own block](/getting-started/first-block/).
