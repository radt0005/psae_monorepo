+++
title = "Remote Sensing"
weight = 1
description = "Processing satellite imagery at scale, including reprojection, tiling, and analysis."
+++

Spade is ideal for remote sensing workflows that need to process large volumes of satellite imagery efficiently.

## The Challenge

Remote sensing pipelines typically involve:

- Downloading imagery from multiple satellite providers
- Reprojecting data into consistent coordinate systems
- Tiling large rasters into manageable chunks
- Running analysis algorithms across thousands of tiles
- Combining results into final products

These workflows are inherently parallel but traditionally require complex orchestration code.

## How Spade Helps

With Spade, you define each processing step as an independent block and wire them together in a YAML pipeline. The scheduler automatically:

- **Parallelizes** tile processing across all available workers
- **Caches** intermediate results so re-runs skip completed steps
- **Handles failures** gracefully with per-block error isolation
- **Scales** from a laptop to a cluster without pipeline changes

## Example Pipeline

A typical remote sensing pipeline in Spade might look like:

1. **Fetch** — Download imagery using a data provider block
2. **Reproject** — Transform to the target CRS using the GDAL reproject block
3. **Tile** — Split into a collection of tiles using a map block
4. **Analyze** — Process each tile in parallel
5. **Merge** — Reduce results back into a single output
