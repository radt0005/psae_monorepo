+++
title = "Parallel Processing with Map/Reduce"
description = "Process large datasets in parallel using map and reduce blocks."
weight = 3
+++

This tutorial walks through building a complete map/reduce pipeline. You will process a large satellite scene that is too big to handle as a single image by splitting it into tiles, processing each tile independently in parallel, and then combining the results back into a single output.

By the end of this tutorial you will understand:

- When to use map/reduce instead of a regular pipeline
- How to create a map block that produces an expansion manifest
- How downstream blocks automatically fan out across tiles
- How broadcast inputs deliver shared data to every parallel invocation
- How a reduce block collects parallel results into a single output

## The problem

You have a large satellite scene covering an entire metropolitan region. You want to classify land cover in the scene using a pre-trained machine learning model. The challenge is that the full scene is too large to process at once -- it would exceed memory limits. The solution is to split the scene into smaller tiles, classify each tile independently, and then stitch the classified tiles back into a single mosaic.

This is a textbook map/reduce pattern:

1. **Map** -- Split the scene into N tiles
2. **Process** -- Classify each tile (N parallel invocations)
3. **Reduce** -- Mosaic the N classified tiles into one output

## Step 1: Understand the three block kinds

Spade blocks have a `kind` field in their manifest that determines how the scheduler handles them:

- **`standard`** -- Runs once per pipeline invocation. This is the default for most blocks.
- **`map`** -- Runs once and produces an **expansion** -- a manifest listing N items. The scheduler then creates N parallel invocations of each downstream block.
- **`reduce`** -- Runs once, after all parallel invocations complete. Receives a **collection** of N items as input and produces a single combined result.

The map block does not process the tiles itself. It only enumerates them. The actual processing happens in standard blocks that the scheduler automatically fans out.

## Step 2: Create the map block

The map block takes a large scene as input and splits it into tiles. Its manifest must declare `kind: map` and have at least one output of type `expansion`.

Here is the manifest for `raster.tile` at `blocks/tile.yaml`:

```yaml
id: raster.tile
version: 1.0.0
kind: map
network: false
description: >
  Splits a large raster into fixed-size tiles and produces an
  expansion manifest listing each tile for parallel processing.

entrypoint: tile

inputs:
  scene:
    type: file
    format: GeoTIFF
    description: The large input raster to split into tiles
  tile_size:
    type: number
    description: Width and height of each tile in pixels

outputs:
  tile:
    type: expansion
    format: GeoTIFF
    description: Individual tiles for downstream processing
```

The key difference from a standard block is:

- **`kind: map`** instead of `kind: standard`
- The output type is **`expansion`** instead of `file`

### How the map block's handler works

The handler does two things: (1) writes the actual tile files and (2) writes an `expansion.yaml` manifest listing them.

Here is a simplified Python implementation:

```python
import os
import numpy as np
import rasterio
from rasterio.windows import Window
import yaml
from spade import run, RasterFile


def handler(scene: RasterFile, tile_size: int) -> None:
    """Split a raster into tiles and write the expansion manifest."""
    with rasterio.open(scene.path) as src:
        width = src.width
        height = src.height
        profile = src.profile.copy()

        items = []
        tile_dir = "outputs/tile"
        os.makedirs(tile_dir, exist_ok=True)

        tile_index = 0
        for row_start in range(0, height, tile_size):
            for col_start in range(0, width, tile_size):
                # Define the window for this tile
                win_height = min(tile_size, height - row_start)
                win_width = min(tile_size, width - col_start)
                window = Window(col_start, row_start, win_width, win_height)

                # Read and write the tile
                tile_data = src.read(window=window)
                tile_filename = f"tile_{tile_index:04d}.tif"
                tile_path = os.path.join(tile_dir, tile_filename)

                tile_profile = profile.copy()
                tile_profile.update(
                    width=win_width,
                    height=win_height,
                    transform=rasterio.windows.transform(window, src.transform),
                )
                with rasterio.open(tile_path, "w", **tile_profile) as dst:
                    dst.write(tile_data)

                # Add to the expansion manifest
                items.append({
                    "path": f"tile/{tile_filename}",
                    "key": f"{tile_index:04d}",
                })
                tile_index += 1

    # Write the expansion manifest
    expansion = {"items": items}
    with open(os.path.join(tile_dir, "expansion.yaml"), "w") as f:
        yaml.dump(expansion, f)


if __name__ == "__main__":
    run(handler)
```

The expansion manifest (`expansion.yaml`) looks like this after the block runs:

```yaml
items:
  - path: tile/tile_0000.tif
    key: "0000"
  - path: tile/tile_0001.tif
    key: "0001"
  - path: tile/tile_0002.tif
    key: "0002"
  - path: tile/tile_0003.tif
    key: "0003"
  # ... one entry per tile
```

Each item has:

- **`path`** -- The file path for this item, relative to the map block's output directory
- **`key`** -- A unique string identifier for this item, used to label the parallel invocations

The scheduler reads this manifest and creates one invocation of each downstream block per item.

## Step 3: Connect standard processing blocks

After the map block, add the blocks that should run once per tile. These are ordinary `kind: standard` blocks -- they do not need any special map/reduce logic. The scheduler handles the fan-out automatically.

In our scenario, we want to compute NDVI for each tile and then classify it. The blocks connect via bare references, just like in a regular pipeline:

```yaml
  # Compute NDVI for each tile (runs N times, once per tile)
  - id: 019d3000-0004-7000-0000-000000000000
    name: raster-tools.ndvi
    inputs:
      - 019d3000-0003-7000-0000-000000000000
    args:
      nodata_value: -9999
```

Because `raster-tools.ndvi` lists the map block in its `inputs`, it enters **map context**. If the map block produced 16 tiles, Spade creates 16 parallel invocations of `raster-tools.ndvi`. Each invocation receives exactly one tile and produces one NDVI raster. From the block's perspective, nothing is different from a normal run -- it receives a single input file and writes a single output file.

### Chaining blocks in map context

Map context propagates through the dependency chain. If you add another block that depends on `raster-tools.ndvi`, it also runs N times:

```yaml
  # Classify each tile (also runs N times)
  - id: 019d3000-0005-7000-0000-000000000000
    name: ml.classify
    inputs:
      - 019d3000-0004-7000-0000-000000000000
      - 019d3000-0002-7000-0000-000000000000
    args:
      confidence_threshold: 0.8
```

This block depends on both `raster-tools.ndvi` (mapped) and `data.download-model` (not mapped). Spade handles this correctly:

- The NDVI input is **mapped**: each of the N invocations receives a different NDVI raster (the one corresponding to its tile).
- The model input is **broadcast**: every invocation receives the **same** model file.

Broadcasting happens automatically whenever a block in map context depends on a block outside the map context. You do not need to configure it explicitly.

## Step 4: Add the broadcast input

The classification block needs a pre-trained model. This model is the same for every tile -- it is not part of the map context. A standard block outside the map produces it:

```yaml
  # Download the classification model (runs once, shared by all tiles)
  - id: 019d3000-0002-7000-0000-000000000000
    name: data.download-model
    inputs: []
    args:
      model_name: "landcover-v3"
      format: "onnx"
```

This block runs once and produces a single model file. When `ml.classify` lists it in its `inputs`, every parallel invocation of `ml.classify` receives the same model file. The model is **broadcast** -- copied (actually symlinked) into each invocation's `inputs/` directory.

Here is what happens for tile 5, for example:

```
019d3000-0005-7000-0000-000000000000.5/
  inputs/
    ndvi_raster -> .../019d3000-0004-7000-0000-000000000000.5/outputs/ndvi_raster/ndvi.tif
    model       -> .../019d3000-0002-7000-0000-000000000000/outputs/model/landcover-v3.onnx
  outputs/
  logs/
```

The NDVI input comes from the corresponding tile's invocation (`.5`), while the model comes from the single, non-mapped model download block.

## Step 5: Add the reduce block

The reduce block collects all N classified tiles and combines them into a single mosaic. Its manifest declares `kind: reduce` and has at least one input of type `collection`:

```yaml
id: raster.mosaic
version: 1.0.0
kind: reduce
network: false
description: >
  Combines a collection of raster tiles into a single mosaic raster.

entrypoint: mosaic

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

Key differences from a standard block:

- **`kind: reduce`** instead of `kind: standard`
- The input type is **`collection`** with an **`item_type`** field, instead of `file`

The `item_type: file` tells Spade that each item in the collection is a single file. The scheduler gathers all N outputs from the upstream mapped block and presents them to the reduce block as numbered files:

```
inputs/
  tiles/
    001.tif    (classified tile 0)
    002.tif    (classified tile 1)
    003.tif    (classified tile 2)
    ...
    016.tif    (classified tile 15)
```

The reduce block's handler reads all files in the collection and combines them:

```python
import os
import numpy as np
import rasterio
from rasterio.merge import merge
from spade import run, RasterFileCollection, RasterFile


def handler(tiles: RasterFileCollection) -> RasterFile:
    """Mosaic a collection of raster tiles into a single output."""
    # Open all tile datasets
    datasets = [rasterio.open(path) for path in sorted(tiles.paths)]

    # Merge into a single raster
    mosaic_data, mosaic_transform = merge(datasets)

    # Write the output
    profile = datasets[0].profile.copy()
    profile.update(
        width=mosaic_data.shape[2],
        height=mosaic_data.shape[1],
        transform=mosaic_transform,
    )

    output_path = "outputs/mosaic/mosaic.tif"
    os.makedirs("outputs/mosaic", exist_ok=True)
    with rasterio.open(output_path, "w", **profile) as dst:
        dst.write(mosaic_data)

    # Clean up
    for ds in datasets:
        ds.close()

    return RasterFile(path=output_path)


if __name__ == "__main__":
    run(handler)
```

Notice how the handler receives a `RasterFileCollection` instead of a single `RasterFile`. The collection has a `.paths` attribute containing a list of all tile file paths. The handler processes all of them together to produce a single output.

## Step 6: The complete pipeline

Here is the full pipeline assembled:

```yaml
id: 019d3000-0000-7000-0000-000000000000
name: parallel-classification
version: "1.0"
description: >
  Download a large satellite scene, split it into tiles, compute
  NDVI and classify each tile in parallel using a shared model,
  then mosaic the results into a single output.

blocks:
  # ---- Source blocks (run once, no dependencies) ----

  # Download the satellite scene
  - id: 019d3000-0001-7000-0000-000000000000
    name: data.download-scene
    inputs: []
    args:
      region: "POLYGON((-97.8 30.2, -97.6 30.2, -97.6 30.4, -97.8 30.4, -97.8 30.2))"
      date_range: "2025-06-01/2025-06-30"
      bands: ["B04", "B05"]

  # Download the pre-trained classification model
  - id: 019d3000-0002-7000-0000-000000000000
    name: data.download-model
    inputs: []
    args:
      model_name: "landcover-v3"
      format: "onnx"

  # ---- Map: split the scene into tiles ----

  - id: 019d3000-0003-7000-0000-000000000000
    name: raster.tile
    inputs:
      - 019d3000-0001-7000-0000-000000000000
    args:
      tile_size: 512

  # ---- Parallel processing (one invocation per tile) ----

  # Compute NDVI for each tile
  - id: 019d3000-0004-7000-0000-000000000000
    name: raster-tools.ndvi
    inputs:
      - 019d3000-0003-7000-0000-000000000000
    args:
      nodata_value: -9999

  # Classify each tile using the broadcast model
  - id: 019d3000-0005-7000-0000-000000000000
    name: ml.classify
    inputs:
      - 019d3000-0004-7000-0000-000000000000
      - 019d3000-0002-7000-0000-000000000000
    args:
      confidence_threshold: 0.8
      classes: ["water", "vegetation", "bare-soil", "urban"]

  # ---- Reduce: mosaic all classified tiles ----

  - id: 019d3000-0006-7000-0000-000000000000
    name: raster.mosaic
    inputs:
      - 019d3000-0005-7000-0000-000000000000
    args:
      method: "nearest"
      output_crs: "EPSG:32614"
```

## Step 7: Walk through the execution

When you run this pipeline, here is exactly what happens:

### Phase 1: Source blocks

`data.download-scene` and `data.download-model` have no dependencies, so they run in parallel.

```
  [1/6] data.download-scene ....... done (12.1s)
  [2/6] data.download-model ....... done (3.4s)
```

### Phase 2: Map

`raster.tile` runs after the scene download completes. It splits the scene into tiles and writes the expansion manifest. Suppose it produces 16 tiles.

```
  [3/6] raster.tile ............... done (2.3s)  [16 tiles]
```

### Phase 3: Parallel processing

The scheduler reads the expansion manifest and creates 16 invocations of `raster-tools.ndvi`, one per tile. They run in parallel (limited by the number of available workers):

```
  [4/6] raster-tools.ndvi ......... done (16 invocations, 8.7s)
```

Then 16 invocations of `ml.classify` run, each receiving:
- A different NDVI raster (mapped input, one per tile)
- The same model file (broadcast input, shared across all invocations)

```
  [5/6] ml.classify ............... done (16 invocations, 14.2s)
```

### Phase 4: Reduce

After all 16 classify invocations complete, `raster.mosaic` runs once. It receives a collection of 16 classified tiles and combines them:

```
  [6/6] raster.mosaic ............. done (3.1s)
Pipeline complete! (43.8s total)
```

### Execution diagram

```
data.download-scene ----> raster.tile --+--> ndvi (tile 0) --> classify (tile 0)  --+
                                        |                                           |
data.download-model ------broadcast-----+--> ndvi (tile 1) --> classify (tile 1)  --+
                                        |                                           +--> raster.mosaic
                                        +--> ndvi (tile 2) --> classify (tile 2)  --+
                                        |                                           |
                                        +--> ...           --> ...                --+
                                        |                                           |
                                        +--> ndvi (tile 15) -> classify (tile 15) --+
```

## Step 8: Validate and run

Validate the pipeline:

```bash
spade check parallel-classification.yaml
```

Spade checks map/reduce-specific constraints in addition to the standard validation rules:

- Map blocks must have at least one `expansion` output
- Reduce blocks must have at least one `collection` input
- Every map context must eventually be terminated by a reduce block
- Nested maps (a map block inside another map's context) are not allowed

Run the pipeline:

```bash
spade run parallel-classification.yaml
```

To control parallelism:

```bash
spade run --workers 8 parallel-classification.yaml
```

The `--workers` flag limits how many block invocations run simultaneously. With 16 tiles and 8 workers, Spade processes 8 tiles at a time.

## Key concepts to remember

**Map blocks enumerate, they do not process.** The map block's job is to split the input and write the expansion manifest. The actual per-item processing happens in downstream standard blocks.

**Standard blocks in map context are unaware of the fan-out.** A block like `raster-tools.ndvi` does not know it is running 16 times. Each invocation receives a single input and produces a single output, just like a normal run.

**Broadcast is automatic.** If a block in map context depends on a block outside the map context, the outside block's output is automatically available to every parallel invocation. No special configuration is needed.

**Reduce blocks collect everything.** The reduce block waits for all parallel invocations to complete, then receives all their outputs as a numbered collection. It produces a single combined result.

**Nested maps are not supported.** You cannot have a map block inside another map block's context. If you need multi-level parallelism, use a reduce block to flatten the first level before mapping again in a separate pipeline.

## Next steps

- Learn about [testing blocks](/tutorials/testing-blocks/) including how to test map and reduce blocks locally
- See the [Map/Reduce concept page](/concepts/map-reduce/) for the full specification
- Review [Map/Reduce Pipelines](/pipelines/map-reduce-pipelines/) for the pipeline YAML reference
- Explore [Pipeline Examples](/pipelines/examples/) for additional patterns
