+++
title = "Quickstart"
description = "Create your first Python block step by step."
weight = 1
+++

This guide walks you through creating a working Spade block in Python from scratch. By the end, you will have a block that reads a GeoTIFF raster file and writes a JSON summary of its properties. The entire block is a plain Python function -- no framework classes to inherit, no lifecycle hooks to implement.

## Prerequisites

Before starting, make sure you have:

- **Python 3.12 or later** installed
- **`uv`** (recommended) or `pip` for dependency management
- The **Spade CLI** installed ([Installation guide](/getting-started/installation/))

## Step 1: Create a block collection

A block collection is a repository that groups related blocks together. All blocks in a collection share the same language. Create a new directory and scaffold it as a Python collection:

```bash
mkdir raster-tools && cd raster-tools
spade init -l python
```

This produces:

```
raster-tools/
  pyproject.toml
  src/
    raster_tools/
      __init__.py
  blocks/
```

The `blocks/` directory is where manifest YAML files live. The `src/raster_tools/` directory is where your Python handler code goes. Note that Spade converts hyphens in the directory name to underscores for the Python package name.

## Step 2: Add a block

Use the CLI to scaffold a new block:

```bash
spade add info
```

This creates two files:

1. **`blocks/info.yaml`** -- A manifest declaring the block's interface (inputs, outputs, parameters)
2. **`src/raster_tools/info.py`** -- A stub handler function you will fill in

## Step 3: Define the manifest

Edit `blocks/info.yaml` to describe what your block accepts and produces:

```yaml
id: raster-tools.info
version: 0.1.0
kind: standard
network: false
description: Reads a raster file and produces a JSON summary of its properties.

entrypoint: src/raster_tools/info.py

inputs:
  raster:
    type: file
    format: GeoTIFF
    description: The input raster file to inspect
  include_stats:
    type: boolean
    description: Whether to compute per-band statistics

outputs:
  summary:
    type: json
    description: JSON file with raster metadata and optional statistics
```

This manifest tells Spade that the block:

- Expects a GeoTIFF file input named `raster`
- Accepts a boolean parameter named `include_stats`
- Produces a JSON output named `summary`
- Does not need network access
- Is a standard (non-map, non-reduce) block

## Step 4: Implement the handler

Replace the contents of `src/raster_tools/info.py` with:

```python
import json
from spade import run, RasterFile, JsonFile


def handler(raster: RasterFile, include_stats: bool) -> JsonFile:
    """Reads a raster file and produces a JSON summary of its properties."""
    from osgeo import gdal

    ds = gdal.Open(raster.path)
    info = {
        "width": ds.RasterXSize,
        "height": ds.RasterYSize,
        "band_count": ds.RasterCount,
        "projection": ds.GetProjection(),
        "geotransform": list(ds.GetGeoTransform()),
    }

    if include_stats:
        band_stats = []
        for i in range(1, ds.RasterCount + 1):
            band = ds.GetRasterBand(i)
            stats = band.GetStatistics(True, True)
            band_stats.append({
                "band": i,
                "min": stats[0],
                "max": stats[1],
                "mean": stats[2],
                "stddev": stats[3],
            })
        info["band_stats"] = band_stats

    ds = None  # Close the dataset

    output_path = "outputs/summary/info.json"
    with open(output_path, "w") as f:
        json.dump(info, f, indent=2)

    return JsonFile(path=output_path)


if __name__ == "__main__":
    run(handler)
```

There are a few things to notice here:

- **The function is just a regular Python function.** There is no base class to inherit from, no decorators required, and no framework-specific patterns to learn.
- **Type hints drive everything.** `raster: RasterFile` tells the library to look for the file-based input named `raster` in the `inputs/` directory and wrap it as a `RasterFile` object (which has a `.path` attribute). `include_stats: bool` tells the library to read that value from `params.yaml`.
- **The return type** `JsonFile` indicates the output is a JSON file. You write the file yourself and return a `JsonFile` pointing to it.
- **`run(handler)` is the entry point.** It reads `params.yaml`, scans `inputs/`, builds the function arguments, calls your handler, and writes outputs. This single line is the only Spade-specific wiring.

## Step 5: Validate

Check that the manifest and source file are consistent:

```bash
spade check
```

Expected output:

```
Collection 'raster-tools' (python) is valid.
  1 block found: raster-tools.info
```

## Step 6: Install and test locally

Install the collection from the local directory:

```bash
spade install file://.
```

This builds the Python package and registers the block in `~/.spade/blocks/raster-tools/0.1.0/`.

You can now reference `raster-tools.info` in any pipeline YAML file.

## Step 7: Use in a pipeline

Create a pipeline YAML file that uses your block:

```yaml
name: raster-info-test
version: "1.0"
description: Test the raster info block

blocks:
  - id: "@info"
    name: raster-tools.info
    inputs: []
    args:
      include_stats: true
```

Run it with:

```bash
spade run raster-info-test.yaml
```

## Recap

The full workflow is:

1. `spade init -l python` -- scaffold a collection
2. `spade add <name>` -- add a block (creates manifest + source file)
3. Edit the manifest to declare inputs and outputs
4. Implement the handler as a typed Python function
5. `spade check` -- validate
6. `spade install file://.` -- build and register locally
7. Reference the block in a pipeline and run it

## Next steps

- [Types](/libraries/python/types/) -- all available Spade types for inputs and outputs
- [Handler Functions](/libraries/python/handlers/) -- patterns for writing handler functions
- [Manifest Generation](/libraries/python/manifest-generation/) -- auto-generate manifests from type hints
- [Examples](/libraries/python/examples/) -- complete worked examples
