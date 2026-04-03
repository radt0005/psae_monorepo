+++
title = "Map/Reduce"
description = "Parallel processing for collections of data using fan-out and collection patterns."
weight = 4
+++

Many data processing tasks involve applying the same operation to a large number of similar items: classifying hundreds of satellite image tiles, processing thousands of sensor readings, or running a model on every parcel in a region. Spade's **map/reduce** system handles these workloads by fanning out work across many parallel invocations and then collecting the results.

## When to use map/reduce

Use map/reduce when:

- You have a **collection of similar items** that need the same processing (e.g., tiles, time steps, regions)
- The processing for each item is **independent** of the others
- You need to **aggregate** or **combine** the results afterward

If you only have a fixed number of inputs that are known at pipeline design time, standard blocks connected in a regular pipeline are simpler and sufficient. Map/reduce is for situations where the number of items is determined at runtime.

## The three-step pattern

Map/reduce in Spade follows a three-step pattern:

1. **Map block** — Enumerates the items to process. It produces an **expansion** output that tells the scheduler how many items there are and where to find each one.
2. **Downstream blocks** — Run once per item. The scheduler automatically creates N parallel invocations, one for each item from the map block.
3. **Reduce block** — Collects all the individual results into a single output.

Here is how data flows through the pattern:

```
[Map block]
    |
    |  expansion (N items)
    |
    v
[Process block] x N   (one invocation per item)
    |
    |  N outputs
    |
    v
[Reduce block]
    |
    |  combined result
    v
```

## Map blocks

A map block has `kind: map` in its manifest and at least one output of type `expansion`:

```yaml
id: tiles.enumerate
version: 1.0.0
kind: map
network: true
description: Downloads a tile index for a region and enumerates individual tiles

entrypoint: src/tiles/enumerate.py

inputs:
  region:
    type: string
    description: WKT polygon defining the area of interest
  zoom:
    type: number
    description: Tile zoom level

outputs:
  tile:
    type: expansion
    format: GeoTIFF
    description: Individual satellite image tiles to process
```

### The expansion manifest

When a map block runs, it writes an `expansion.yaml` file in its output directory. This file tells the scheduler what items to fan out over. The format is:

```yaml
items:
  - path: tiles/tile_001.tif
    key: "001"
  - path: tiles/tile_002.tif
    key: "002"
  - path: tiles/tile_003.tif
    key: "003"
  - path: tiles/tile_004.tif
    key: "004"
```

Each item has:

- **`path`** — The file path for this item, relative to the map block's output directory
- **`key`** — A unique string identifier for this item. The key is used to label the resulting invocations and track them through the pipeline.

The map block is responsible for writing both the `expansion.yaml` manifest and the actual data files it references.

## How the scheduler fans out

When the scheduler encounters a map block's expansion output, it creates **N invocations** of each downstream block, one per item. These invocations are named using the downstream block's ID with a numeric suffix:

```
process-tile.0    (receives tile_001.tif)
process-tile.1    (receives tile_002.tif)
process-tile.2    (receives tile_003.tif)
process-tile.3    (receives tile_004.tif)
```

Each invocation receives exactly one item from the expansion as its input. All N invocations run independently and can execute in parallel.

### Context propagation

Blocks downstream of a mapped block **inherit the map context**. This means if block A is mapped and block B depends on block A, block B also runs N times, one for each map item. Block B's invocation `process-tile.2` receives the output from block A's invocation `process-tile.2`, and so on. The map context flows through the entire chain until a reduce block collects the results.

### Broadcasting non-mapped inputs

Often, a downstream block needs both a per-item input (from the map) and a shared input that is the same for every invocation. For example, a classification block might receive a different image tile for each invocation but the same trained model for all of them.

Spade handles this automatically. If a downstream block has inputs from both a map block (which gets fanned out) and a standard block (which runs once), the standard block's output is **broadcast** to every invocation:

```yaml
blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: tiles.enumerate
    inputs: []
    args:
      region: "POLYGON((-105.5 40.0, -105.0 40.0, -105.0 40.5, -105.5 40.5, -105.5 40.0))"
      zoom: 14

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: ml.train
    inputs: []
    args:
      model_type: random_forest

  - id: 019cf4bc-3333-7000-0000-000000000000
    name: ml.classify
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
      - 019cf4bc-2222-7000-0000-000000000000
    args:
      confidence_threshold: 0.8
```

In this example, `ml.classify` runs N times (once per tile from `tiles.enumerate`). Each invocation receives a different tile as input, but all invocations receive the same trained model from `ml.train`.

## Reduce blocks

A reduce block has `kind: reduce` in its manifest and at least one input of type `collection` with an `item_type` field:

```yaml
id: raster.mosaic
version: 1.0.0
kind: reduce
network: false
description: Combines classified tiles into a single mosaic raster

entrypoint: src/raster/mosaic.py

inputs:
  tiles:
    type: collection
    item_type: file
    format: GeoTIFF
    description: The classified tiles to mosaic together

outputs:
  mosaic:
    type: file
    format: GeoTIFF
    description: The combined mosaic raster
```

The `item_type` field tells Spade what kind of data each item in the collection is (e.g., `file`, `directory`). The scheduler collects the outputs from all N invocations of the upstream mapped block and presents them to the reduce block as a numbered collection:

```
inputs/tiles/001.tif   (output from process-tile.0)
inputs/tiles/002.tif   (output from process-tile.1)
inputs/tiles/003.tif   (output from process-tile.2)
inputs/tiles/004.tif   (output from process-tile.3)
```

The reduce block processes all items together and produces a single combined output.

## Limitations

**Nested maps are not yet supported.** You cannot have a map block whose downstream blocks include another map block. If you need multi-level fan-out, split your workflow into separate pipelines or restructure so that a single map block enumerates all items at the finest granularity.

## Complete walkthrough example

Here is a full pipeline that uses map/reduce to process satellite imagery:

```yaml
id: 019cf4bc-0000-7000-0000-000000000000
name: land-cover-classification
version: "1.0"
description: >
  Download satellite tiles for a region, classify each tile using
  a pre-trained model, and mosaic the results into a single map.

blocks:
  # Step 1: Enumerate tiles for the region (map block)
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: tiles.enumerate
    inputs: []
    args:
      region: "POLYGON((-105.5 40.0, -105.0 40.0, -105.0 40.5, -105.5 40.5, -105.5 40.0))"
      zoom: 14

  # Step 2: Train or load a classification model (standard block)
  - id: 019cf4bc-2222-7000-0000-000000000000
    name: ml.train
    inputs: []
    args:
      model_type: random_forest
      training_data: s3://my-bucket/training-samples.geojson

  # Step 3: Preprocess each tile (runs N times, once per tile)
  - id: 019cf4bc-3333-7000-0000-000000000000
    name: raster.normalize
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
    args:
      method: min_max

  # Step 4: Classify each preprocessed tile (runs N times)
  #         Receives a different tile from raster.normalize AND
  #         the same model from ml.train (broadcast)
  - id: 019cf4bc-4444-7000-0000-000000000000
    name: ml.classify
    inputs:
      - 019cf4bc-3333-7000-0000-000000000000
      - 019cf4bc-2222-7000-0000-000000000000
    args:
      confidence_threshold: 0.8

  # Step 5: Mosaic all classified tiles into one raster (reduce block)
  - id: 019cf4bc-5555-7000-0000-000000000000
    name: raster.mosaic
    inputs:
      - 019cf4bc-4444-7000-0000-000000000000
    args:
      nodata_value: -9999
```

What happens when this pipeline runs:

1. **`tiles.enumerate`** (map) downloads the tile index and writes an expansion manifest listing, say, 12 tiles.
2. **`ml.train`** (standard) runs in parallel with step 1, producing a trained model.
3. **`raster.normalize`** runs 12 times (once per tile), preprocessing each tile independently.
4. **`ml.classify`** runs 12 times. Each invocation receives a different preprocessed tile from step 3 and the same trained model from step 2 (broadcast).
5. **`raster.mosaic`** (reduce) collects all 12 classified tiles and combines them into a single output raster.

Steps 3 and 4 each produce 12 parallel invocations, making efficient use of available compute resources.
