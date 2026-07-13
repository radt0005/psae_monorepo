+++
title = "Map/Reduce Pipelines"
description = "Writing pipelines that use parallel fan-out and collection patterns."
weight = 5
+++

Some processing tasks involve applying the same operation to many items independently -- for example, processing each tile of a large satellite image, analyzing each file in a dataset, or running a model on each sample. Spade supports this pattern through **map/reduce pipelines**, where a map block fans out work to parallel invocations and a reduce block collects the results.

## Overview of the pattern

A map/reduce pipeline follows three stages:

1. **Map**: A map block takes a single input and produces an **expansion** -- a manifest listing multiple items to process. Spade reads this expansion and creates one parallel invocation of each downstream block per item.

2. **Process**: Downstream blocks connected to the map block run once per expanded item, in parallel. Each invocation receives one item from the expansion. These blocks are ordinary blocks -- they do not need any special map-aware logic.

3. **Reduce**: A reduce block collects the outputs from all parallel invocations and combines them into a single result. It receives a **collection** input containing all the individual outputs.

```
              +---> process (item 0) ---+
              |                         |
source ---> map ---> process (item 1) ---+--> reduce ---> output
              |                         |
              +---> process (item 2) ---+
```

## Declaring a map block invocation

A map block is a block whose manifest declares `kind: map`. In the pipeline, you invoke it like any other block. The key difference is in what it produces: instead of a regular output, it writes an **expansion manifest** -- a YAML file listing the items to fan out over.

```yaml
blocks:
  - id: "@scene"
    name: data.download-scene
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"

  - id: "@tile"
    name: raster.tile
    inputs:
      - "@scene"
    args:
      tile_size: 256
```

In this example, `raster.tile` is a map block. Given a large raster, it splits it into 256x256 pixel tiles and writes an expansion manifest listing each tile as a separate item. Spade reads this manifest and creates parallel invocations of any downstream blocks -- one per tile.

## Connecting downstream blocks in map context

Any block that lists a map block in its `inputs` automatically enters **map context**. Spade creates one invocation of the downstream block for each item in the expansion. Each invocation receives the corresponding item as its input.

From the downstream block's perspective, nothing is different -- it receives a single input and produces a single output, just like any non-mapped block. The parallelism is handled entirely by the Spade scheduler.

```yaml
  - id: "@ndvi"
    name: raster.ndvi
    inputs:
      - "@tile"
    args:
      red_band: 4
      nir_band: 8
```

If `raster.tile` produced 12 tiles, Spade creates 12 parallel invocations of `raster.ndvi`, each processing one tile. The invocations are identified by appending an index to the resolved block invocation UUID: `@ndvi.0`, `@ndvi.1`, and so on.

You can chain multiple blocks in map context. If another block depends on `raster.ndvi`, it also runs once per tile:

```yaml
  - id: "@classify"
    name: raster.classify
    inputs:
      - "@ndvi"
    args:
      threshold: 0.3
```

This creates another 12 parallel invocations, each receiving the NDVI output for its corresponding tile.

## Broadcasting non-mapped inputs

Sometimes a block in map context needs both a mapped input (one per item) and a shared input that is the same for every invocation. This is called **broadcasting**.

If a block in map context lists multiple inputs, Spade distinguishes between:

- **Mapped inputs**: Inputs that come from an upstream block in the same map context. Each invocation gets a different item.
- **Broadcast inputs**: Inputs that come from an upstream block outside the map context. Every invocation gets the same data.

```yaml
blocks:
  # Source: download a pre-trained classification model (not mapped)
  - id: "@model"
    name: data.download-model
    inputs: []
    args:
      model_name: "landcover-v2"

  # Source: download the satellite scene
  - id: "@scene"
    name: data.download-scene
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"

  # Map: split into tiles
  - id: "@tile"
    name: raster.tile
    inputs:
      - "@scene"
    args:
      tile_size: 256

  # Process each tile: classify using the shared model
  - id: "@classify"
    name: raster.classify
    inputs:
      - "@tile"    # mapped: one tile per invocation
      - "@model"   # broadcast: same model for all
    args:
      threshold: 0.3
```

In this example, `raster.classify` runs once per tile. Each invocation receives its own tile (mapped from the expansion) plus the same classification model (broadcast from the non-mapped download block).

Spade determines automatically which inputs are mapped and which are broadcast based on whether the upstream block is inside or outside the map context.

## Reduce blocks collecting results

A reduce block is a block whose manifest declares `kind: reduce`. It collects the outputs from all parallel invocations in a map context and produces a single combined result.

In the pipeline, the reduce block lists the last mapped block in its `inputs`. Spade gathers all the parallel outputs into a **collection** and passes them to the reduce block as a single input.

```yaml
  - id: "@mosaic"
    name: raster.mosaic
    inputs:
      - "@classify"
    args:
      method: "nearest"
```

The `raster.mosaic` block is a reduce block. It receives a collection of all classified tiles and combines them into a single output raster. The reduce block runs exactly once, after all parallel invocations of the upstream block have completed.

Inside the reduce block's handler, the input is a collection type (e.g., `RasterFileCollection` in Python, which provides a list of file paths) rather than a single file.

## Complete example: satellite tile processing

Below is a complete end-to-end pipeline that demonstrates the full map/reduce pattern. The pipeline:

1. Downloads a satellite scene
2. Downloads a pre-trained land cover classification model
3. Splits the scene into tiles (map)
4. Computes NDVI for each tile (parallel, mapped)
5. Classifies each tile using the shared model (parallel, mapped + broadcast)
6. Mosaics the classified tiles back together (reduce)

```yaml
name: tile-classification
version: "1.0"
description: >
  Download a satellite scene, split it into tiles, compute NDVI,
  classify each tile with a pre-trained model, and mosaic the
  results back together.

blocks:

  # ---- Sources (no dependencies) ----

  # Download the satellite imagery
  - id: "@scene"
    name: data.download-scene
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"
      date_range: "2025-06-01/2025-06-30"
      bands: ["B04", "B08"]

  # Download the pre-trained classification model
  - id: "@model"
    name: data.download-model
    inputs: []
    args:
      model_name: "landcover-v2"
      format: "onnx"

  # ---- Map: split the scene into tiles ----

  - id: "@tile"
    name: raster.tile
    inputs:
      - "@scene"
    args:
      tile_size: 256
      overlap: 16

  # ---- Parallel processing (one invocation per tile) ----

  # Compute NDVI for each tile
  - id: "@ndvi"
    name: raster.ndvi
    inputs:
      - "@tile"
    args:
      red_band: 4
      nir_band: 8

  # Classify each tile using the broadcast model
  - id: "@classify"
    name: raster.classify
    inputs:
      - "@ndvi"    # mapped: NDVI for this tile
      - "@model"   # broadcast: shared model
    args:
      threshold: 0.3
      classes: ["water", "vegetation", "bare-soil", "urban"]

  # ---- Reduce: mosaic all classified tiles ----

  - id: "@mosaic"
    name: raster.mosaic
    inputs:
      - "@classify"
    args:
      method: "nearest"
      output_crs: "EPSG:4326"
```

The execution flow is:

1. `data.download-scene` and `data.download-model` run in parallel (no dependencies on each other).
2. `raster.tile` runs after the scene download completes. It produces an expansion of N tiles.
3. `raster.ndvi` runs N times in parallel, once per tile.
4. `raster.classify` runs N times in parallel, once per tile. Each invocation receives its tile's NDVI output (mapped) and the shared model (broadcast).
5. `raster.mosaic` runs once after all classify invocations complete. It receives a collection of all N classified tiles and produces a single output mosaic.

## Nested map/reduce

A map block may itself sit inside another map block's context, giving you multi-level fan-out -- for example, enumerate scenes, then enumerate tiles within each scene, process each tile, mosaic per scene, then combine all scenes. Nesting requires no special YAML: it falls out of which blocks depend on which. Downstream invocation IDs gain one index component per enclosing map level (`@classify.1.4` is tile 4 of scene 1), and inner reduce blocks run once per outer item rather than once for the whole pipeline. Nesting is capped at 4 levels deep, since invocation counts multiply at each level.

See [Nested map/reduce](/concepts/map-reduce/#nested-mapreduce) for the full mechanics (ragged fan-out, broadcasting by context depth, well-nestedness) and a worked YAML example.

## Constraints and limitations

- **Reduce blocks must have `kind: reduce` in their manifest.** An ordinary block cannot receive a collection input from a map context -- Spade will report a type error during validation.
- **Broadcast inputs must come from outside the map context they're feeding, or from an enclosing context.** A block cannot broadcast an input from a sibling invocation inside its own context.
- **Every map context must be closed by a matching reduce**, and contexts must be well-nested -- you cannot combine the outputs of two sibling unclosed contexts. See [Pipeline Validation](/pipelines/validation/) for the exact rules `spade check` enforces.
- **All parallel invocations in a context must complete before that context's reduce block runs.** There is no partial reduction or streaming behavior.
