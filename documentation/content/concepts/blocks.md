+++
title = "Blocks"
description = "Self-contained units of computation that form the building blocks of Spade pipelines."
weight = 1
+++

A **block** is the fundamental unit of work in Spade. Each block is a self-contained program that reads inputs, performs a computation, and writes outputs. Blocks run as isolated subprocesses inside a sandbox powered by `isolate`, which restricts what files, memory, and CPU the block can access. This isolation means blocks cannot interfere with each other or with the host system, making pipelines safe and reproducible.

## What blocks do

At a high level, a block:

1. Reads input data from its `inputs/` directory and parameters from `params.yaml`
2. Performs some computation (data transformation, analysis, model inference, etc.)
3. Writes results to its `outputs/` directory

Blocks are language-agnostic. You can write a block in Python, R, TypeScript, Go, or Rust. The Spade runtime only cares about the block's **manifest** (a YAML file declaring its interface) and that the block's entrypoint is executable.

## The block manifest

Every block has a manifest file located at `blocks/<name>.yaml` within its [collection](/concepts/collections/). The manifest declares the block's identity, what it accepts, and what it produces.

Here is a complete example:

```yaml
id: raster.reproject
version: 0.2.1
kind: standard
network: false
description: Reprojects a raster file to a target coordinate reference system using GDAL.

entrypoint: src/raster/reproject.py

inputs:
  raster:
    type: file
    format: GeoTIFF
    description: The input raster file to reproject
  target_crs:
    type: string
    description: Target coordinate reference system (e.g., "EPSG:4326")
  resolution:
    type: number
    description: Output resolution in target CRS units (optional)

outputs:
  reprojected:
    type: file
    format: GeoTIFF
    description: The reprojected raster file
```

## Field reference

### Top-level fields

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique identifier in the form `<collection>.<block>`. This is how pipelines reference the block. |
| `version` | Yes | Semantic version string (e.g., `0.1.0`, `1.2.3`). Used for caching and installation. |
| `kind` | No | One of `standard`, `map`, or `reduce`. Determines how the scheduler handles the block. Defaults to `standard`. See [Map/Reduce](/concepts/map-reduce/) for details on `map` and `reduce` kinds. |
| `network` | No | Boolean. Whether the block needs network access. Defaults to `false`. |
| `description` | No | Human-readable description of what the block does. |
| `entrypoint` | No | Path to the executable file, relative to the collection root, or a subcommand/entry-point name. This is what Spade runs inside the sandbox. Defaults to the manifest's filename stem (e.g. `rasterize.yaml` → the `rasterize` subcommand or `rasterize.py`/`rasterize.R` script). |
| `inputs` | Yes | A map of named inputs the block expects. |
| `outputs` | Yes | A map of named outputs the block produces. |

{% note() %}
`kind` and `entrypoint` are optional for `standard` blocks, which can rely entirely on the defaults above. To create a **map** or **reduce** block, you must explicitly set `kind: map` or `kind: reduce` (there's no way to opt into fan-out behavior by default) and explicitly declare `entrypoint`, since map and reduce entry points typically don't follow the same filename-stem convention as a standard block's. See [Map/Reduce](/concepts/map-reduce/) for the full pattern.
{% end %}

### Input fields

Each entry under `inputs` is a named input with the following fields:

| Field | Required | Description |
|-------|----------|-------------|
| `type` | Yes | The data type of the input. See [Input types](#input-types) below. |
| `format` | No | A hint about the file format (e.g., `GeoTIFF`, `CSV`, `JSON`). Used for documentation and type matching. |
| `description` | No | Human-readable description of this input. |

### Output fields

Each entry under `outputs` is a named output with the following fields:

| Field | Required | Description |
|-------|----------|-------------|
| `type` | Yes | The data type of the output. See [Output types](#output-types) below. |
| `format` | No | A hint about the file format. |
| `description` | No | Human-readable description of this output. |

## Input types

Blocks can declare inputs of the following types:

### `file`

A single file. Spade places the file at `inputs/<name>` in the block's working directory. Use this for any input that is a single file, such as a raster image, a CSV table, or a trained model.

### `directory`

A directory of files. Spade places the entire directory at `inputs/<name>/`. Use this when your block needs a set of related files that should remain together, such as a shapefile (which consists of multiple files like `.shp`, `.dbf`, `.shx`, `.prj`).

### `collection`

A variable-length sequence of files. Spade places the files at `inputs/<name>/001.tif`, `inputs/<name>/002.tif`, and so on, with zero-padded numeric filenames. Use this when the number of input items varies between runs, such as a set of image tiles. See [Collections](#collections) below for more detail.

### `string`

A text value. Provided via the pipeline's `args` and written into `params.yaml`. Use this for configuration values like coordinate reference systems, column names, or file paths on external systems.

### `number`

A numeric value (integer or floating-point). Provided via `args` and written into `params.yaml`. Use this for thresholds, resolutions, or other numeric parameters.

### `boolean`

A true/false value. Provided via `args` and written into `params.yaml`. Use this for feature flags or toggles that control block behavior.

## Output types

Blocks can declare outputs of the following types:

### `file`

A single output file. The block writes it to `outputs/<name>`. This is the most common output type.

### `directory`

A directory of output files. The block writes them under `outputs/<name>/`. Use this when the block produces a set of related files that belong together.

### `collection`

A variable-length sequence of output files. The block writes them as `outputs/<name>/001.tif`, `outputs/<name>/002.tif`, etc. Use this when the block produces a dynamic number of output items.

### `json`

A JSON file output. The block writes a JSON file to `outputs/<name>`. This is functionally similar to `file` but signals to downstream blocks and the runtime that the content is structured JSON data.

### `expansion`

A special output type used only by [map blocks](/concepts/map-reduce/). The block writes an `expansion.yaml` manifest that tells the scheduler how to fan out downstream blocks. This type is not used by standard or reduce blocks.

## Collections

Collections (in the input/output type sense, not to be confused with [block collections](/concepts/collections/)) handle variable-length data. When a block declares an input or output of type `collection`, files are numbered sequentially with zero-padded indices:

```
inputs/tiles/001.tif
inputs/tiles/002.tif
inputs/tiles/003.tif
...
```

This convention allows blocks to process any number of items without knowing the count in advance. The block simply reads all files in the directory, sorted by name.

For outputs, the block writes files using the same numbering scheme into `outputs/<name>/`.

## The execution environment

When Spade runs a block, it creates an isolated working directory with a specific layout:

```
<working-directory>/
  params.yaml          # Parameters from the pipeline's args
  invocation.yaml      # Metadata about this invocation (block ID, run ID, etc.)
  inputs/              # Input data from upstream blocks (symlinked)
    <input-name>       # One entry per declared input
  outputs/             # Where the block writes its results
    <output-name>      # One entry per declared output
  logs/                # Captured stdout and stderr
    stdout.log
    stderr.log
```

### params.yaml

Contains the key-value pairs from the pipeline's `args` for this block invocation. For example, if the pipeline specifies:

```yaml
args:
  target_crs: "EPSG:4326"
  resolution: 30
```

Then `params.yaml` will contain:

```yaml
target_crs: "EPSG:4326"
resolution: 30
```

Your block code reads this file to get its parameters. The Spade libraries for each language handle this automatically.

### inputs/

Contains the block's input data. For inputs that come from upstream blocks, Spade creates symlinks pointing to the upstream block's output files. For example, if your block has an input named `raster` that comes from an upstream block's output named `reprojected`, then `inputs/raster` is a symlink to that file.

### outputs/

This is where your block writes its results. Each declared output should be written to `outputs/<output-name>`. Spade checks that all declared outputs exist after the block finishes.

### logs/

Spade captures the block's standard output and standard error into `logs/stdout.log` and `logs/stderr.log`. These are preserved even if the block fails, which makes them essential for debugging.

### invocation.yaml

Contains machine-generated metadata about the current invocation:

```yaml
block:
  id: raster.reproject
  version: 1.0.0
invocation_id: 01HZX...
inputs:
  reference:
    path: inputs/reference/data.tif
    hash: abc123
  target:
    path: inputs/target/data.tif
    hash: def456
```

This includes the block's own `id` and `version`, the invocation ID, and -- per declared input -- the resolved path and a content hash. Most blocks do not need to read this file, but it is available for advanced use cases (for example, logging exactly which input version produced a given output). Blocks may read `invocation.yaml` but should not modify it.

## Caching

Spade caches block outputs to avoid redundant computation. When a block is about to run, Spade computes a **cache key** from:

- The block's `id`
- The block's `version`
- A content hash of every input file
- The contents of `params.yaml`
- A hash of the runtime environment (language version, installed dependencies)

If a matching cache entry exists, Spade skips execution and reuses the cached outputs. This means that changing any input, parameter, or the block version itself will trigger re-execution, but re-running an identical pipeline is nearly instantaneous.

## Network access

By default, blocks run with **no network access**. This is enforced by the `isolate` sandbox. Blocks that need to download data from external services (such as a satellite imagery provider or a database) must declare `network: true` in their manifest:

```yaml
network: true
```

Only enable network access when the block genuinely needs it. Keeping network disabled for most blocks improves reproducibility and security.

## What blocks must not do

The execution model described above only works if blocks stick to it. Specifically, a block must not:

- **Access files outside its working directory.** The sandbox enforces this, but design your block to only ever touch `params.yaml`, `inputs/`, `outputs/`, and `invocation.yaml`.
- **Discover inputs dynamically.** Don't scan for files or infer what data is available -- read only the named inputs declared in the manifest.
- **Assume original filenames.** An input's path inside `inputs/<name>/` is not guaranteed to match the filename it had before Spade placed it there.
- **Write outside `outputs/`.** Anything a downstream block or the pipeline result needs must be written under `outputs/<name>/`.
- **Depend on global or external mutable state.** A block's output should be a pure function of its declared inputs, parameters, and version -- this is what makes caching correct. Reading from a database or API that can change between runs is fine, but be aware it breaks the assumption that identical inputs produce identical (cacheable) outputs.
- **Access the network unless `network: true` is declared.** The sandbox blocks it by default; declaring the flag is also how the network requirement becomes visible to anyone reviewing the pipeline.

## Error handling

If a block's subprocess exits with a **non-zero exit code**, Spade treats the invocation as a failure. When any block fails:

1. The pipeline **halts** immediately. No further blocks are scheduled.
2. The block's **logs are preserved** in the working directory for inspection.
3. Spade reports which block failed and its exit code.

To debug a failed block, check `logs/stdout.log` and `logs/stderr.log` in the block's working directory. You can also use `spade run --keep-work-dir` to preserve the full working directory after a failure.

## Complete manifest example

Here is a more detailed manifest showing all available fields:

```yaml
id: ml.classify
version: 1.0.0
kind: standard
network: false
description: >
  Classifies land cover in a satellite image using a pre-trained
  random forest model. Produces a classified raster and a JSON
  report with class distributions.

entrypoint: src/ml/classify.py

inputs:
  image:
    type: file
    format: GeoTIFF
    description: Multi-band satellite image to classify
  model:
    type: file
    format: pickle
    description: Trained scikit-learn model file
  labels:
    type: file
    format: JSON
    description: JSON mapping of class IDs to human-readable labels
  confidence_threshold:
    type: number
    description: Minimum confidence score to assign a class (0.0 to 1.0)
  include_probabilities:
    type: boolean
    description: Whether to include per-pixel probability bands in the output

outputs:
  classified:
    type: file
    format: GeoTIFF
    description: Single-band raster with class IDs as pixel values
  report:
    type: json
    description: JSON report with class distribution statistics
```
