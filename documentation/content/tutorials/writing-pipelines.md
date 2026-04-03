+++
title = "Writing Pipelines"
description = "Design and build a multi-step processing pipeline from scratch."
weight = 2
+++

This tutorial walks through designing and building a multi-step geospatial processing pipeline from scratch. You will start with a research question, identify the processing steps needed, choose the right blocks, write the pipeline YAML, handle tricky wiring situations, validate, and run.

By the end of this tutorial you will understand:

- How to break a research workflow into pipeline steps
- How to wire blocks together using `inputs` and `args`
- When to use bare references versus explicit references
- How to validate and debug a pipeline
- How to iterate on pipeline design

## The scenario

You are a researcher studying **urban heat islands** -- the phenomenon where cities are significantly warmer than surrounding rural areas. You have a study area (a metropolitan region) and you want to produce a map showing surface temperature differences overlaid with land cover information.

Your analysis requires these steps:

1. **Download satellite imagery** for the study area (Landsat thermal and multispectral bands)
2. **Compute land surface temperature (LST)** from the thermal band
3. **Compute NDVI** from the red and NIR bands (vegetation correlates with cooler temperatures)
4. **Classify land cover** (urban, vegetation, water, bare soil) from the multispectral image
5. **Generate a composite report** combining LST, NDVI, and land cover into a single multi-layer output

## Step 1: Identify the processing steps

Before writing any YAML, sketch out the data flow. Each step becomes a block invocation in the pipeline. Think about:

- **What data does each step need?** This determines the `inputs` for each block.
- **What does each step produce?** This determines which blocks can be wired together.
- **Which steps depend on which?** This determines the execution order.
- **Which steps are independent?** Independent steps can run in parallel.

Here is the dependency graph for our analysis:

```
                              +--> raster.lst --------+
                              |                       |
data.landsat --+--> raster.split-bands --+            |
               |              |          |            |
               |              +--> raster-tools.ndvi -+--> analysis.heat-island
               |                                      |
               +--> ml.classify ----- ----------------+
```

Key observations:

- `data.landsat` is the only source block (no dependencies).
- `raster.split-bands` and `ml.classify` both depend on `data.landsat`, so they can run in parallel after the download completes.
- `raster.lst` depends on the thermal band from `raster.split-bands`.
- `raster-tools.ndvi` depends on the red and NIR bands from `raster.split-bands`.
- `analysis.heat-island` depends on outputs from `raster.lst`, `raster-tools.ndvi`, and `ml.classify`, so it runs last.

## Step 2: Write the pipeline header

Create a file called `heat-island.yaml`. Start with the pipeline metadata:

```yaml
id: 019d2000-0000-7000-0000-000000000000
name: urban-heat-island
version: "1.0"
description: >
  Analyze urban heat island effects by computing land surface
  temperature, NDVI, and land cover classification from Landsat
  imagery, then combining results into a composite report.

blocks:
```

The `id` is a UUIDv7 that uniquely identifies this pipeline. The `name` is a human-readable label that appears in CLI output and logs. The `version` is a string (note the quotes around `"1.0"` -- YAML would otherwise interpret it as a number).

## Step 3: Add the source block

The first block downloads the satellite imagery. It has no upstream dependencies, so its `inputs` list is empty:

```yaml
blocks:
  # Step 1: Download Landsat imagery for the study area
  - id: 019d2000-0001-7000-0000-000000000000
    name: data.landsat
    inputs: []
    args:
      region: "POLYGON((-97.8 30.2, -97.6 30.2, -97.6 30.4, -97.8 30.4, -97.8 30.2))"
      date_range: "2025-06-01/2025-08-31"
      bands: ["B04", "B05", "B10"]
```

Important details:

- **`inputs: []`** -- An empty list, not omitted entirely. This tells Spade the block is a source node with no dependencies.
- **`args`** -- These scalar parameters are written to `params.yaml` when the block runs. The block's handler reads them to know what data to fetch.

## Step 4: Add independent processing branches

Next, add the blocks that depend only on the download step. Since `raster.split-bands` and `ml.classify` are independent of each other, Spade will run them in parallel once `data.landsat` completes.

```yaml
  # Step 2: Split into individual bands
  - id: 019d2000-0002-7000-0000-000000000000
    name: raster.split-bands
    inputs:
      - 019d2000-0001-7000-0000-000000000000
    args:
      red_band: 4
      nir_band: 5
      thermal_band: 10

  # Step 3: Classify land cover (runs in parallel with band splitting)
  - id: 019d2000-0003-7000-0000-000000000000
    name: ml.classify
    inputs:
      - 019d2000-0001-7000-0000-000000000000
    args:
      model_type: random_forest
      classes: ["urban", "vegetation", "water", "bare-soil"]
```

Both blocks reference the download block's ID (`019d2000-0001-7000-0000-000000000000`) in their `inputs`. These are **bare references** -- just the invocation ID string. Bare references work when the type matching is unambiguous. In both cases, the download block produces one raster output, and each downstream block expects one raster input.

## Step 5: Add downstream processing

Now add the blocks that depend on the band-splitting step:

```yaml
  # Step 4: Compute land surface temperature from thermal band
  - id: 019d2000-0004-7000-0000-000000000000
    name: raster.lst
    inputs:
      - block: 019d2000-0002-7000-0000-000000000000
        output: thermal
        as: thermal_band
    args:
      emissivity: 0.95

  # Step 5: Compute NDVI from red and NIR bands
  - id: 019d2000-0005-7000-0000-000000000000
    name: raster-tools.ndvi
    inputs:
      - block: 019d2000-0002-7000-0000-000000000000
        output: red
        as: red_band
      - block: 019d2000-0002-7000-0000-000000000000
        output: nir
        as: nir_band
    args:
      nodata_value: -9999
```

Notice that both blocks use **explicit references** instead of bare references. This is necessary because `raster.split-bands` produces **multiple outputs of the same type** (all are GeoTIFF files). A bare reference would be ambiguous -- Spade would not know which band goes to which input. Explicit references solve this by naming the exact output.

Each explicit reference has three parts:

- **`block`** -- The invocation ID of the upstream block
- **`output`** -- The name of the specific output on that block
- **`as`** -- The name of the input on the downstream block to connect it to

The `as` field is optional when the downstream block has only one compatible input, but including it makes the wiring explicit and easier to understand when reading the pipeline.

## Step 6: Add the final merge step

The last block combines all the intermediate results into a single composite output:

```yaml
  # Step 6: Generate heat island analysis report
  - id: 019d2000-0006-7000-0000-000000000000
    name: analysis.heat-island
    inputs:
      - 019d2000-0004-7000-0000-000000000000
      - 019d2000-0005-7000-0000-000000000000
      - 019d2000-0003-7000-0000-000000000000
    args:
      output_format: geotiff
      include_legend: true
```

This block depends on three upstream blocks. All three are listed as bare references. This works because:

- `raster.lst` produces a temperature raster (format: GeoTIFF, but semantically a temperature layer)
- `raster-tools.ndvi` produces an NDVI raster
- `ml.classify` produces a classified land cover raster

If `analysis.heat-island` has three inputs of distinct types (temperature, vegetation index, and classification), Spade can match each upstream output to the correct input by type. If the types are ambiguous (for example, if all three upstream outputs have the same type and format), you would need to use explicit references instead.

## Step 7: The complete pipeline

Here is the full `heat-island.yaml` assembled from the sections above:

```yaml
id: 019d2000-0000-7000-0000-000000000000
name: urban-heat-island
version: "1.0"
description: >
  Analyze urban heat island effects by computing land surface
  temperature, NDVI, and land cover classification from Landsat
  imagery, then combining results into a composite report.

blocks:
  # Step 1: Download Landsat imagery for the study area
  - id: 019d2000-0001-7000-0000-000000000000
    name: data.landsat
    inputs: []
    args:
      region: "POLYGON((-97.8 30.2, -97.6 30.2, -97.6 30.4, -97.8 30.4, -97.8 30.2))"
      date_range: "2025-06-01/2025-08-31"
      bands: ["B04", "B05", "B10"]

  # Step 2: Split into individual bands (red, NIR, thermal)
  - id: 019d2000-0002-7000-0000-000000000000
    name: raster.split-bands
    inputs:
      - 019d2000-0001-7000-0000-000000000000
    args:
      red_band: 4
      nir_band: 5
      thermal_band: 10

  # Step 3: Classify land cover (parallel with Steps 4-5)
  - id: 019d2000-0003-7000-0000-000000000000
    name: ml.classify
    inputs:
      - 019d2000-0001-7000-0000-000000000000
    args:
      model_type: random_forest
      classes: ["urban", "vegetation", "water", "bare-soil"]

  # Step 4: Compute land surface temperature
  - id: 019d2000-0004-7000-0000-000000000000
    name: raster.lst
    inputs:
      - block: 019d2000-0002-7000-0000-000000000000
        output: thermal
        as: thermal_band
    args:
      emissivity: 0.95

  # Step 5: Compute NDVI
  - id: 019d2000-0005-7000-0000-000000000000
    name: raster-tools.ndvi
    inputs:
      - block: 019d2000-0002-7000-0000-000000000000
        output: red
        as: red_band
      - block: 019d2000-0002-7000-0000-000000000000
        output: nir
        as: nir_band
    args:
      nodata_value: -9999

  # Step 6: Combine into heat island analysis
  - id: 019d2000-0006-7000-0000-000000000000
    name: analysis.heat-island
    inputs:
      - 019d2000-0004-7000-0000-000000000000
      - 019d2000-0005-7000-0000-000000000000
      - 019d2000-0003-7000-0000-000000000000
    args:
      output_format: geotiff
      include_legend: true
```

## Step 8: Validate the pipeline

Before running, always validate:

```bash
spade check heat-island.yaml
```

If everything is correct:

```
Pipeline 'urban-heat-island' is valid.
  6 blocks, 0 errors.
```

### Fixing common validation errors

**"Block 'raster.lst' is not installed"** -- You need to install the block collection first:

```bash
spade install https://github.com/example/raster-blocks.git
```

**"Ambiguous input resolution"** -- The type matcher cannot determine which output goes to which input. Replace the bare reference with explicit references that name the exact output and input. See Step 5 above for an example.

**"Missing required argument 'emissivity'"** -- A parameter declared in the block's manifest is not provided in the pipeline's `args`. Add the missing key-value pair to the `args` map.

**"Duplicate block invocation ID"** -- Two blocks have the same `id`. Generate a new UUIDv7 for one of them.

**"Dependency cycle detected"** -- There is a circular dependency. Restructure the pipeline so data flows in one direction, from sources to sinks.

## Step 9: Run the pipeline

Execute the pipeline locally:

```bash
spade run heat-island.yaml
```

Spade resolves the dependency graph and executes blocks in the correct order. Blocks that are independent of each other run in parallel:

```
Running pipeline 'urban-heat-island'...
  [1/6] data.landsat .............. done (8.4s)
  [2/6] raster.split-bands ........ done (1.2s)
  [3/6] ml.classify ............... done (3.7s)
  [4/6] raster.lst ................ done (0.9s)
  [5/6] raster-tools.ndvi ......... done (0.8s)
  [6/6] analysis.heat-island ...... done (2.1s)
Pipeline complete! (17.1s total)
```

Steps 2 and 3 run in parallel after Step 1 completes. Steps 4 and 5 run in parallel after Step 2 completes. Step 6 waits for Steps 3, 4, and 5 to all finish.

### Inspecting results

To keep the working directory after the pipeline finishes (so you can examine intermediate outputs):

```bash
spade run --keep-work-dir heat-island.yaml
```

Each block's working directory contains `inputs/`, `outputs/`, and `logs/` subdirectories. You can inspect the final output in the last block's `outputs/` directory, or check `logs/stderr.log` for any warnings.

### Re-running with caching

If you run the pipeline again without changing anything, Spade uses cached results:

```
Running pipeline 'urban-heat-island'...
  [1/6] data.landsat .............. (cached)
  [2/6] raster.split-bands ........ (cached)
  [3/6] ml.classify ............... (cached)
  [4/6] raster.lst ................ (cached)
  [5/6] raster-tools.ndvi ......... (cached)
  [6/6] analysis.heat-island ...... (cached)
Pipeline complete! (0.3s total)
```

If you change a parameter (for example, adjusting `emissivity` from `0.95` to `0.97`), only the affected block and its downstream dependents re-execute. The upstream blocks are served from cache.

## Step 10: Iterate on the design

Pipelines are rarely perfect on the first try. Here are common ways to iterate:

### Adding a new step

Suppose you want to add a reprojection step after downloading. Insert a new block invocation and update the downstream references:

```yaml
  # New step: Reproject to UTM
  - id: 019d2000-0007-7000-0000-000000000000
    name: raster.reproject
    inputs:
      - 019d2000-0001-7000-0000-000000000000
    args:
      target_crs: "EPSG:32614"
```

Then update `raster.split-bands` and `ml.classify` to reference the reproject block (`019d2000-0007-...`) instead of the download block (`019d2000-0001-...`).

### Changing parameters

Adjusting parameters in `args` is the simplest change. Only the affected block and its downstream dependents re-execute. Everything upstream is served from cache.

### Replacing a block

If you find a better NDVI implementation, change the `name` field from `raster-tools.ndvi` to the new block's name. Make sure the new block's input and output types are compatible, then validate with `spade check`.

## Guidelines for pipeline design

**Keep blocks small and focused.** Each block should do one thing well. A block that downloads data, computes NDVI, and generates a report is doing too much. Split it into three blocks so each step can be cached, tested, and reused independently.

**List blocks in topological order.** The order of blocks in the YAML file does not affect execution -- Spade determines the order from the dependency graph. However, listing source blocks first and sink blocks last makes the pipeline much easier to read.

**Start with bare references.** Use bare references for simplicity. Switch to explicit references only when `spade check` reports an ambiguity, or when you want to be extra clear about the wiring.

**Use descriptive arg names.** Since `args` become `params.yaml` inside the block, descriptive names like `emissivity` and `cloud_probability_threshold` make the pipeline self-documenting.

**Version your pipelines.** Increment the pipeline `version` when you make significant changes. This helps track which version of the pipeline produced which results.

## Next steps

- Learn about [parallel processing with map/reduce](/tutorials/map-reduce-tutorial/) for pipelines that process many items
- Explore [testing strategies](/tutorials/testing-blocks/) for developing and debugging blocks
- See the [Pipeline Examples](/pipelines/examples/) page for more patterns
- Read the [Input References](/pipelines/input-references/) guide for the full details on bare and explicit references
