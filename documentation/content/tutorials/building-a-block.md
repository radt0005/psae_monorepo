+++
title = "Building a Block"
description = "End-to-end guide to creating a custom processing block."
weight = 1
+++

This tutorial walks through building a complete Spade block from scratch. You will create a block that computes the **Normalized Difference Vegetation Index (NDVI)** from satellite imagery -- a common remote sensing operation that measures vegetation health. NDVI is defined as:

```
NDVI = (NIR - Red) / (NIR + Red)
```

where NIR is the near-infrared band and Red is the visible red band. The result ranges from -1 to +1, with higher values indicating healthier vegetation.

By the end of this tutorial you will have:

1. Planned the block's inputs and outputs
2. Written the block manifest (YAML)
3. Implemented the handler function (Python)
4. Validated the block with `spade check`
5. Installed the block locally
6. Used the block in a pipeline

## Prerequisites

Before starting, make sure you have:

- The Spade CLI installed ([Installation guide](/getting-started/installation/))
- Python 3.12 or later
- `uv` package manager (recommended) or `pip`
- `numpy` and `rasterio` Python packages (we will add these as dependencies)

## Step 1: Plan the block's interface

Before writing any code, decide what the block accepts and what it produces. This is the most important design step -- it determines how the block fits into pipelines.

For an NDVI computation, we need:

**Inputs:**
- A **red band** raster file (GeoTIFF) -- the visible red reflectance
- A **NIR band** raster file (GeoTIFF) -- the near-infrared reflectance

**Outputs:**
- An **NDVI raster** file (GeoTIFF) -- a single-band raster with NDVI values

We also want a parameter to control what value to assign to pixels where the computation is undefined (both bands are zero). This is the **nodata value**, and we will make it a number parameter.

**Parameters (via args):**
- `nodata_value` (number) -- the value to write where NDVI is undefined (default: `-9999`)

{% note() %}
**Why two separate file inputs instead of one multi-band image?** Keeping the inputs as separate files makes the block more flexible. It can receive bands from different upstream blocks, or from a single block that splits a multi-band image into individual bands. This modularity is a core principle of Spade block design.
{% end %}

## Step 2: Create the block collection

A block collection is a repository that groups related blocks together. All blocks in a collection share the same programming language.

Create a new collection for raster processing blocks:

```bash
mkdir raster-tools && cd raster-tools
spade init --language python
```

This scaffolds a Python project:

```
raster-tools/
  pyproject.toml
  src/
    raster_tools/
      __init__.py
  blocks/
```

## Step 3: Add the block

Use `spade add` to create the manifest and source file:

```bash
spade add ndvi
```

This creates two files:

```
  Created blocks/ndvi.yaml
  Created src/raster_tools/ndvi.py
```

## Step 4: Write the block manifest

Open `blocks/ndvi.yaml` and replace its contents with the interface you planned in Step 1:

```yaml
id: raster-tools.ndvi
version: 0.1.0
kind: standard
network: false
description: >
  Computes the Normalized Difference Vegetation Index (NDVI) from
  red and near-infrared raster bands. NDVI = (NIR - Red) / (NIR + Red).

entrypoint: ndvi

inputs:
  red_band:
    type: file
    format: GeoTIFF
    description: Visible red reflectance band
  nir_band:
    type: file
    format: GeoTIFF
    description: Near-infrared reflectance band
  nodata_value:
    type: number
    description: Value to assign where NDVI is undefined (both bands zero)

outputs:
  ndvi_raster:
    type: file
    format: GeoTIFF
    description: Single-band NDVI raster with values from -1 to 1
```

Let's walk through each field:

- **`id: raster-tools.ndvi`** -- Unique identifier following the `<collection>.<block>` convention. This is how pipelines reference the block.
- **`version: 0.1.0`** -- Semantic version. Changing this invalidates cached results.
- **`kind: standard`** -- This is a regular processing block, not a map or reduce block.
- **`network: false`** -- The block does not need internet access. It only reads local files.
- **`entrypoint: ndvi`** -- The name used to locate the source file. For Python, Spade looks for `src/raster_tools/ndvi.py`.
- **`inputs`** -- Two file inputs (`red_band` and `nir_band`) and one numeric parameter (`nodata_value`). File inputs come from upstream blocks; the numeric parameter comes from the pipeline's `args`.
- **`outputs`** -- One file output (`ndvi_raster`) containing the computed NDVI.

## Step 5: Implement the handler

Open `src/raster_tools/ndvi.py` and replace the stub with the actual implementation:

```python
import numpy as np
import rasterio
from spade import run, RasterFile


def handler(red_band: RasterFile, nir_band: RasterFile, nodata_value: float) -> RasterFile:
    """Compute NDVI from red and NIR raster bands.

    NDVI = (NIR - Red) / (NIR + Red)
    """
    # Read the input raster bands
    with rasterio.open(red_band.path) as red_src:
        red = red_src.read(1).astype(np.float64)
        profile = red_src.profile.copy()

    with rasterio.open(nir_band.path) as nir_src:
        nir = nir_src.read(1).astype(np.float64)

    # Compute NDVI, handling division by zero
    denominator = nir + red
    ndvi = np.where(
        denominator != 0,
        (nir - red) / denominator,
        nodata_value,
    )

    # Update the raster profile for single-band float output
    profile.update(dtype=rasterio.float64, count=1, nodata=nodata_value)

    # Write the output
    output_path = "outputs/ndvi_raster/ndvi.tif"
    with rasterio.open(output_path, "w", **profile) as dst:
        dst.write(ndvi, 1)

    return RasterFile(path=output_path)


if __name__ == "__main__":
    run(handler)
```

There are several important things to understand about this code:

### How inputs are loaded

The function signature tells the Spade library how to load each argument:

- **`red_band: RasterFile`** -- Spade looks in `inputs/red_band/` for a file and creates a `RasterFile` object with a `.path` attribute pointing to it.
- **`nir_band: RasterFile`** -- Same pattern, reads from `inputs/nir_band/`.
- **`nodata_value: float`** -- Spade reads this from `params.yaml` (which is generated from the pipeline's `args`).

The parameter names in the function signature **must match** the input names in the manifest. `red_band` in the manifest maps to `red_band` in the function.

### How outputs are written

The block writes its output file to `outputs/ndvi_raster/ndvi.tif`. The directory name `ndvi_raster` **must match** the output name in the manifest. Spade expects to find the output at `outputs/<output_name>/` after the block finishes.

The function returns a `RasterFile` pointing to the output path. The `run()` function uses this to verify the output exists.

### The entry point

The `if __name__ == "__main__": run(handler)` block at the bottom is what Spade calls when it executes the block. The `run()` function:

1. Reads `params.yaml` for scalar parameters
2. Scans `inputs/` for file-based inputs
3. Calls your handler with the loaded arguments
4. Verifies the output was written

## Step 6: Add dependencies

Your block uses `numpy` and `rasterio`. Add them to `pyproject.toml`:

```toml
[project]
name = "raster-tools"
version = "0.1.0"
requires-python = ">=3.12"
dependencies = [
    "spade",
    "numpy",
    "rasterio",
]
```

If you are using `uv`:

```bash
uv add numpy rasterio
```

## Step 7: Validate with spade check

Run the validation command from the collection root:

```bash
spade check
```

If everything is correct, you will see:

```
Collection is valid. 1 block(s) checked.
```

`spade check` verifies:

- The manifest has all required fields (`id`, `version`, `inputs`, `outputs`)
- Input and output types are valid
- The entrypoint file exists at `src/raster_tools/ndvi.py`
- The block ID follows the `<collection>.<block>` convention

If there is a problem -- for example, a missing field or a typo in the entrypoint -- the error message will tell you exactly what to fix.

## Step 8: Install locally

Install the collection from the local directory:

```bash
spade install file://.
```

This runs `uv sync` to install dependencies, then copies the built artifacts and manifests to `~/.spade/blocks/raster-tools/0.1.0/`. The block is now available for use in pipelines.

```
Cloning file://.
Detected language: python
Collection: raster-tools v0.1.0
Building...
Installed 1 block(s) to /home/user/.spade/blocks/raster-tools/0.1.0
```

## Step 9: Use in a pipeline

Now create a pipeline that uses your NDVI block. In a real scenario, the red and NIR bands would come from upstream blocks (such as a band-splitting block). Here is a three-block pipeline that downloads a satellite image, splits it into bands, and computes NDVI:

Create `ndvi-pipeline.yaml`:

```yaml
id: 019d1000-0000-7000-0000-000000000000
name: ndvi-computation
version: "1.0"
description: >
  Download satellite imagery, extract red and NIR bands,
  and compute NDVI.

blocks:
  # Step 1: Download satellite imagery
  - id: 019d1000-0001-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-105.5 40.0, -105.0 40.0, -105.0 40.5, -105.5 40.5, -105.5 40.0))"
      date_range: "2025-06-01/2025-09-01"

  # Step 2: Split into individual bands
  - id: 019d1000-0002-7000-0000-000000000000
    name: raster.split-bands
    inputs:
      - 019d1000-0001-7000-0000-000000000000
    args:
      red_band: 4
      nir_band: 8

  # Step 3: Compute NDVI using our new block
  - id: 019d1000-0003-7000-0000-000000000000
    name: raster-tools.ndvi
    inputs:
      - block: 019d1000-0002-7000-0000-000000000000
        output: red
        as: red_band
      - block: 019d1000-0002-7000-0000-000000000000
        output: nir
        as: nir_band
    args:
      nodata_value: -9999
```

Notice that the NDVI block uses **explicit references** for its inputs. This is necessary because `raster.split-bands` produces two outputs of the same type (both `file` with format `GeoTIFF`). Without explicit references, Spade could not determine which output goes to `red_band` and which goes to `nir_band`. The `output` key names the specific upstream output, and the `as` key names the input on the downstream block.

Validate the pipeline:

```bash
spade check ndvi-pipeline.yaml
```

Then run it:

```bash
spade run ndvi-pipeline.yaml
```

## Notes for other languages

The workflow above uses Python, but the same pattern applies to all supported languages. The manifest YAML is identical regardless of language -- only the handler implementation differs.

**Go:**

```go
package main

import spade "github.com/spade-dev/spade"

func handler(args *spade.Args) (*spade.RasterFile, error) {
    redBand, err := spade.Input[*spade.RasterFile](args, "red_band")
    if err != nil {
        return nil, err
    }
    nirBand, err := spade.Input[*spade.RasterFile](args, "nir_band")
    if err != nil {
        return nil, err
    }
    nodata, err := spade.Param[float64](args, "nodata_value")
    if err != nil {
        return nil, err
    }

    // ... compute NDVI using redBand.Path and nirBand.Path ...

    result := spade.NewRasterFile("outputs/ndvi_raster/ndvi.tif")
    return &result, nil
}

func main() {
    spade.Run(handler)
}
```

**Rust:**

```rust
use spade::{run, Args, RasterFile};

fn handler(args: Args) -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
    let red_band: RasterFile = args.input("red_band")?;
    let nir_band: RasterFile = args.input("nir_band")?;
    let nodata: f64 = args.param("nodata_value")?;

    // ... compute NDVI using red_band.path and nir_band.path ...

    Ok(RasterFile::new("outputs/ndvi_raster/ndvi.tif"))
}

fn main() {
    run(handler);
}
```

**R:**

```r
library(yaml)
library(terra)
library(spade)

params <- read_yaml("params.yaml")

red <- rast("inputs/red_band")
nir <- rast("inputs/nir_band")

ndvi <- (nir - red) / (nir + red)
ndvi[is.nan(ndvi)] <- params$nodata_value

writeRaster(ndvi, "outputs/ndvi_raster/ndvi.tif", overwrite = TRUE)
```

**TypeScript:**

```typescript
import { run, RasterFile } from "spade";

function handler(red_band: RasterFile, nir_band: RasterFile, nodata_value: number): RasterFile {
  // ... compute NDVI using red_band.path and nir_band.path ...

  return new RasterFile("outputs/ndvi_raster/ndvi.tif");
}

run(handler);
```

## Summary

Building a Spade block follows a consistent process:

1. **Plan** -- Decide on inputs, outputs, and parameters before writing code.
2. **Scaffold** -- Use `spade init` and `spade add` to create the collection and block files.
3. **Manifest** -- Declare the block's interface in the YAML manifest. This is the contract between your block and the rest of the pipeline.
4. **Implement** -- Write the handler function. Read from `inputs/`, read parameters from the function arguments, and write results to `outputs/`.
5. **Validate** -- Run `spade check` to catch errors early.
6. **Install** -- Run `spade install file://.` to build and register the block.
7. **Use** -- Reference the block in a pipeline YAML file and run it with `spade run`.

## Next steps

- Learn how to [write multi-step pipelines](/tutorials/writing-pipelines/) using your blocks
- Explore [testing strategies](/tutorials/testing-blocks/) for blocks during development
- Read about [map/reduce](/tutorials/map-reduce-tutorial/) for parallel processing of large datasets
