+++
title = "Pipeline Examples"
description = "Complete, copy-pasteable pipeline YAML examples for common patterns."
weight = 5
+++

This page contains complete, ready-to-use pipeline examples demonstrating common patterns. Each example includes a description of the processing workflow, the full YAML pipeline file, and an explanation of the data flow.

## Example 1: Simple two-block linear pipeline

**Description:** Download a satellite image and reproject it to a different coordinate reference system. This is the simplest possible pipeline -- two blocks connected in sequence.

```yaml
id: 019d0001-0000-7000-0000-000000000000
name: simple-reproject
version: "1.0"
description: Download Sentinel-2 imagery and reproject to EPSG:4326

blocks:
  - id: 019d0001-0001-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"
      date_range: "2025-01-01/2025-06-01"

  - id: 019d0001-0002-7000-0000-000000000000
    name: raster.reproject
    inputs:
      - 019d0001-0001-7000-0000-000000000000
    args:
      target_crs: "EPSG:4326"
```

**Data flow:**

```
data.sentinel2 --> raster.reproject
```

1. `data.sentinel2` runs first. It has no inputs (source block), so it starts immediately. It downloads Sentinel-2 imagery for the specified region and date range, producing a raster output.
2. `raster.reproject` runs after `data.sentinel2` completes. It receives the raster output via a bare reference and reprojects it to EPSG:4326.

The bare reference works here because `data.sentinel2` produces one raster output and `raster.reproject` expects one raster input -- the type match is unambiguous.

---

## Example 2: Parallel branches that merge

**Description:** Download satellite imagery, then run two independent analyses in parallel (NDVI computation and cloud masking), and finally combine the results into a single output.

```yaml
id: 019d0002-0000-7000-0000-000000000000
name: parallel-analysis
version: "1.0"
description: >
  Compute NDVI and a cloud mask in parallel, then combine
  the results into a clean vegetation index product.

blocks:
  # Step 1: Download the satellite scene
  - id: 019d0002-0001-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-105.3 40.0, -105.0 40.0, -105.0 40.3, -105.3 40.3, -105.3 40.0))"
      date_range: "2025-07-01/2025-07-31"

  # Step 2a: Compute NDVI (parallel branch 1)
  - id: 019d0002-0002-7000-0000-000000000000
    name: raster.ndvi
    inputs:
      - 019d0002-0001-7000-0000-000000000000
    args:
      red_band: 4
      nir_band: 8

  # Step 2b: Generate cloud mask (parallel branch 2)
  - id: 019d0002-0003-7000-0000-000000000000
    name: raster.cloud-mask
    inputs:
      - 019d0002-0001-7000-0000-000000000000
    args:
      cloud_probability_threshold: 30

  # Step 3: Apply the cloud mask to the NDVI result
  - id: 019d0002-0004-7000-0000-000000000000
    name: raster.apply-mask
    inputs:
      - 019d0002-0002-7000-0000-000000000000
      - 019d0002-0003-7000-0000-000000000000
    args:
      nodata_value: -9999
```

**Data flow:**

```
                  +--> raster.ndvi -------+
                  |                       |
data.sentinel2 --+                       +--> raster.apply-mask
                  |                       |
                  +--> raster.cloud-mask -+
```

1. `data.sentinel2` runs first as the sole source block.
2. `raster.ndvi` and `raster.cloud-mask` both depend only on `data.sentinel2`. Since they are independent of each other, Spade runs them in parallel.
3. `raster.apply-mask` depends on both `raster.ndvi` and `raster.cloud-mask`. It waits until both complete, then receives both outputs. It uses the cloud mask to mask out cloudy pixels in the NDVI raster.

The bare references from `raster.apply-mask` to its two upstream blocks work because `raster.ndvi` produces a raster (the NDVI image) and `raster.cloud-mask` produces a different type (a mask). Spade can distinguish which output maps to which input based on their types. If both outputs were the same type, you would need [explicit references](/pipelines/input-references/).

---

## Example 3: Map/reduce tile processing pipeline

**Description:** Download a large satellite scene, split it into tiles, process each tile in parallel (compute NDVI and classify), then mosaic the classified tiles back into a single output image. This demonstrates the full [map/reduce pattern](/pipelines/map-reduce-pipelines/).

```yaml
id: 019d0003-0000-7000-0000-000000000000
name: tile-classification
version: "1.0"
description: >
  Split a satellite scene into tiles, compute NDVI and classify
  each tile in parallel, then mosaic the results.

blocks:
  # Download the satellite scene
  - id: 019d0003-0001-7000-0000-000000000000
    name: data.download-scene
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"
      date_range: "2025-06-01/2025-06-30"
      bands: ["B04", "B08"]

  # Download the pre-trained classification model (broadcast input)
  - id: 019d0003-0002-7000-0000-000000000000
    name: data.download-model
    inputs: []
    args:
      model_name: "landcover-v2"

  # Map: split the scene into tiles
  - id: 019d0003-0003-7000-0000-000000000000
    name: raster.tile
    inputs:
      - 019d0003-0001-7000-0000-000000000000
    args:
      tile_size: 256
      overlap: 16

  # Parallel: compute NDVI for each tile
  - id: 019d0003-0004-7000-0000-000000000000
    name: raster.ndvi
    inputs:
      - 019d0003-0003-7000-0000-000000000000
    args:
      red_band: 4
      nir_band: 8

  # Parallel: classify each tile using the shared model
  - id: 019d0003-0005-7000-0000-000000000000
    name: raster.classify
    inputs:
      - 019d0003-0004-7000-0000-000000000000
      - 019d0003-0002-7000-0000-000000000000
    args:
      threshold: 0.3
      classes: ["water", "vegetation", "bare-soil", "urban"]

  # Reduce: mosaic all classified tiles
  - id: 019d0003-0006-7000-0000-000000000000
    name: raster.mosaic
    inputs:
      - 019d0003-0005-7000-0000-000000000000
    args:
      method: "nearest"
      output_crs: "EPSG:4326"
```

**Data flow:**

```
data.download-scene --> raster.tile --+--> raster.ndvi (tile 0) --> raster.classify (tile 0) --+
                                      |                                                        |
data.download-model ----broadcast-----+--> raster.ndvi (tile 1) --> raster.classify (tile 1) --+--> raster.mosaic
                                      |                                                        |
                                      +--> raster.ndvi (tile N) --> raster.classify (tile N) --+
```

1. `data.download-scene` and `data.download-model` run in parallel (both are source blocks with no dependencies).
2. `raster.tile` runs after the scene download completes. It is a map block (`kind: map` in its manifest) that splits the scene into tiles and produces an expansion manifest listing each tile.
3. `raster.ndvi` enters map context because its input comes from the map block. Spade creates one invocation per tile, all running in parallel. Each invocation receives one tile and computes its NDVI.
4. `raster.classify` also runs in map context. Each invocation receives two inputs:
   - The NDVI raster for its specific tile (mapped input from `raster.ndvi`)
   - The classification model (broadcast input from `data.download-model`, shared across all invocations)
5. `raster.mosaic` is a reduce block (`kind: reduce` in its manifest). It waits for all classify invocations to complete, receives a collection of all classified tiles, and produces a single mosaic output.

---

## Example 4: Explicit input references

**Description:** Split a raster into its red and near-infrared bands, then compute a band ratio. Because the upstream block produces two outputs of the same type (both GeoTIFF rasters), bare references would be ambiguous. This example uses [explicit references](/pipelines/input-references/) to wire the correct outputs to the correct inputs.

```yaml
id: 019d0004-0000-7000-0000-000000000000
name: explicit-references
version: "1.0"
description: >
  Demonstrate explicit input references by computing a band ratio
  from individually extracted bands.

blocks:
  # Download multispectral satellite imagery
  - id: 019d0004-0001-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-73.99 40.75, -73.95 40.75, -73.95 40.78, -73.99 40.78, -73.99 40.75))"
      date_range: "2025-08-01/2025-08-31"

  # Extract individual bands from the multispectral image.
  # This block produces two outputs:
  #   - "red"  (type: file, format: GeoTIFF)
  #   - "nir"  (type: file, format: GeoTIFF)
  - id: 019d0004-0002-7000-0000-000000000000
    name: raster.split-bands
    inputs:
      - 019d0004-0001-7000-0000-000000000000
    args:
      red_band: 4
      nir_band: 8

  # Compute the ratio NIR / Red.
  # This block expects two inputs:
  #   - "numerator"   (type: file, format: GeoTIFF)
  #   - "denominator" (type: file, format: GeoTIFF)
  #
  # Both upstream outputs are the same type (GeoTIFF), so a bare
  # reference would be ambiguous. We use explicit references to
  # wire NIR to numerator and Red to denominator.
  - id: 019d0004-0003-7000-0000-000000000000
    name: raster.band-ratio
    inputs:
      - block: 019d0004-0002-7000-0000-000000000000
        output: nir
      - block: 019d0004-0002-7000-0000-000000000000
        output: red
    args: {}

  # Threshold the ratio to produce a binary classification
  - id: 019d0004-0004-7000-0000-000000000000
    name: raster.threshold
    inputs:
      - 019d0004-0003-7000-0000-000000000000
    args:
      threshold: 0.4
      above_value: 1
      below_value: 0
```

**Data flow:**

```
                                    nir output ---> numerator input
data.sentinel2 --> raster.split-bands                                --> raster.band-ratio --> raster.threshold
                                    red output ---> denominator input
```

1. `data.sentinel2` downloads the multispectral imagery.
2. `raster.split-bands` extracts two individual bands from the image. It produces two named outputs: `red` and `nir`, both of type GeoTIFF.
3. `raster.band-ratio` needs both bands as input. Because both outputs have the same type, Spade cannot determine which should be `numerator` and which should be `denominator` using type matching alone. The explicit references solve this:
   - `output: nir` from `raster.split-bands` is wired to the `numerator` input (Spade matches by type after resolving the explicit output name -- since each explicit reference narrows to exactly one output, the remaining type match is unambiguous).
   - `output: red` from `raster.split-bands` is wired to the `denominator` input.
4. `raster.threshold` receives the band ratio result via a bare reference (unambiguous, since `raster.band-ratio` produces one output) and applies a threshold to produce a binary classification.

If you attempted to use bare references for step 3:

```yaml
# THIS WOULD FAIL VALIDATION
inputs:
  - 019d0004-0002-7000-0000-000000000000
```

Spade would report an ambiguity error because two outputs of the same type could be paired with two inputs of the same type in more than one way. The explicit references eliminate the ambiguity.
