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
  - id: "@tiles"
    name: tiles.enumerate
    inputs: []
    args:
      region: "POLYGON((-105.5 40.0, -105.0 40.0, -105.0 40.5, -105.5 40.5, -105.5 40.0))"
      zoom: 14

  - id: "@model"
    name: ml.train
    inputs: []
    args:
      model_type: random_forest

  - id: "@classify"
    name: ml.classify
    inputs:
      - "@tiles"
      - "@model"
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

## Nested map/reduce

A map block can appear inside another map block's context. This lets you fan out at two (or more) levels -- for example, enumerate scenes, then for each scene enumerate its tiles, process each tile, mosaic tiles back into a scene, then combine all scenes into a final result.

### How nesting works

- **The inner map block is itself a mapped block of the outer context.** If the outer map produces N₁ items, the inner map runs N₁ times -- once per outer item -- and each run produces its **own** expansion. The item counts don't have to match across outer items: outer item 0 might expand to 5 inner items while outer item 1 expands to 300. This is called a **ragged** fan-out.
- **The inner reduce is also a mapped block of the outer context.** It runs once per outer item, and each run gathers only the inner invocations belonging to that same outer item -- never across outer items. Only the **outermost** reduce runs exactly once for the whole pipeline.
- **Invocations are identified by an index vector**, one integer per enclosing map level, outermost first. A block nested two levels deep has an invocation ID like `<block-id>.1.4` -- inner item 4 of outer item 1. This makes it possible to trace any invocation back to exactly which item, at every level, produced it.
- **Blocks are nesting-agnostic.** A map block enumerates its input and writes `expansion.yaml` exactly the same way whether it's at the top level or nested two levels deep. You do not write map or reduce blocks any differently to support nesting -- the scheduler and worker handle the per-instance fan-out and reduce readiness.
- **Broadcasting still works, generalized by context depth.** A broadcast input from outside all map contexts is shared by every invocation, as before. A broadcast from an *intermediate* level (for example, a per-scene reference feeding every tile of that scene) is shared by every inner invocation belonging to that specific outer item, but not across outer items.
- **An empty fan-out completes vacuously.** If an inner map instance enumerates zero items, its corresponding inner reduce runs immediately with an empty collection -- the pipeline does not stall waiting for invocations that were never created.

### Rules

- **Every map context must be closed by a reduce.** A nested map that never reaches a matching reduce is a validation error.
- **Contexts must be well-nested.** You cannot combine the outputs of two sibling map contexts unless at least one has already been closed by its reduce -- there's no way to "merge" two open fan-outs directly.
- **Nesting depth is capped at 4 levels.** Invocation counts multiply at each level (N₁ × N₂ × ...), so this bound exists to keep worst-case fan-out from growing unpredictably.
- **Failure semantics are unchanged at any depth.** A failed invocation, however deeply nested, halts the entire pipeline just like a failure at the top level.

### Example

Consider a pipeline `M1 → M2 → X → R2 → R1`, where `M1` (outer map) expands into 2 items, and `M2` (inner map) expands outer item 0 into 3 items and outer item 1 into just 1 item:

```
M1                          runs once   → expansion [a, b]
M2.0, M2.1                  run twice   → expansions [p,q,r] and [s]
X.0.0, X.0.1, X.0.2, X.1.0  run 4x      (ragged: 3 + 1)
R2.0                        runs when X.0.* complete → gathers 3 outputs
R2.1                        runs when X.1.* complete → gathers 1 output
R1                          runs once, when R2.0 and R2.1 have both completed
```

`X` is an ordinary standard block -- it has no idea it's nested two levels deep. It just receives one item and produces one output, exactly like the single-level examples earlier on this page.

### Pipeline YAML for nested map/reduce

Nesting requires no special syntax -- it falls out naturally from which blocks depend on which. Here, `tiles.enumerate` runs inside `scenes.enumerate`'s context because it depends on a mapped block, and `raster.mosaic` closes the inner context before `report.combine` closes the outer one:

```yaml
blocks:
  - id: "@scenes"
    name: scenes.enumerate       # kind: map (outer)
    inputs: []
    args:
      region: "POLYGON((-105.5 40.0, -105.0 40.0, -105.0 40.5, -105.5 40.5, -105.5 40.0))"

  - id: "@tiles"
    name: tiles.enumerate        # kind: map (inner, nested inside @scenes' context)
    inputs:
      - "@scenes"
    args:
      zoom: 14

  - id: "@classify"
    name: ml.classify            # runs once per (scene, tile) pair
    inputs:
      - "@tiles"
    args:
      confidence_threshold: 0.8

  - id: "@mosaic"
    name: raster.mosaic          # kind: reduce (inner) -- closes @tiles' context, once per scene
    inputs:
      - "@classify"

  - id: "@report"
    name: report.combine         # kind: reduce (outer) -- closes @scenes' context, once total
    inputs:
      - "@mosaic"
```

See [Map/Reduce Pipelines](/pipelines/map-reduce-pipelines/) for more on writing these pipelines by hand, and [Pipeline Validation](/pipelines/validation/) for the full set of map/reduce validation rules `spade check` enforces.

## Complete walkthrough example

Here is a full pipeline that uses map/reduce to process satellite imagery:

```yaml
name: land-cover-classification
version: "1.0"
description: >
  Download satellite tiles for a region, classify each tile using
  a pre-trained model, and mosaic the results into a single map.

blocks:
  # Step 1: Enumerate tiles for the region (map block)
  - id: "@tiles"
    name: tiles.enumerate
    inputs: []
    args:
      region: "POLYGON((-105.5 40.0, -105.0 40.0, -105.0 40.5, -105.5 40.5, -105.5 40.0))"
      zoom: 14

  # Step 2: Train or load a classification model (standard block)
  - id: "@model"
    name: ml.train
    inputs: []
    args:
      model_type: random_forest
      training_data: s3://my-bucket/training-samples.geojson

  # Step 3: Preprocess each tile (runs N times, once per tile)
  - id: "@normalize"
    name: raster.normalize
    inputs:
      - "@tiles"
    args:
      method: min_max

  # Step 4: Classify each preprocessed tile (runs N times)
  #         Receives a different tile from raster.normalize AND
  #         the same model from ml.train (broadcast)
  - id: "@classify"
    name: ml.classify
    inputs:
      - "@normalize"
      - "@model"
    args:
      confidence_threshold: 0.8

  # Step 5: Mosaic all classified tiles into one raster (reduce block)
  - id: "@mosaic"
    name: raster.mosaic
    inputs:
      - "@classify"
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
