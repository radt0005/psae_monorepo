+++
title = "Input Resolution"
description = "How Spade matches block outputs to inputs using type matching and explicit references."
weight = 6
+++

When a block lists upstream blocks in its `inputs` array, Spade needs to determine exactly which output from the upstream block connects to which input on the downstream block. This process is called **input resolution**. Spade supports two styles of input references: simple (bare) references and explicit references.

## Simple (bare) references

A bare reference is just the invocation ID of the upstream block:

```yaml
blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-105.5 40.0, -105.0 40.0, -105.0 40.5, -105.5 40.5, -105.5 40.0))"

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.reproject
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
    args:
      target_crs: "EPSG:4326"
```

With a bare reference, Spade automatically matches the upstream block's outputs to the downstream block's inputs by comparing their **types** and **formats**. If `data.sentinel2` produces a single `file` output with format `GeoTIFF` and `raster.reproject` expects a single `file` input with format `GeoTIFF`, Spade connects them.

Bare references work well when the connection is unambiguous: one output matches one input, and there is no confusion about which goes where.

## Explicit references

An explicit reference is an object with `block` and `output` keys that names a specific output on a specific upstream block:

```yaml
blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-105.5 40.0, -105.0 40.0, -105.0 40.5, -105.5 40.5, -105.5 40.0))"

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.composite
    inputs:
      - block: 019cf4bc-1111-7000-0000-000000000000
        output: red_band
        as: band_1
      - block: 019cf4bc-1111-7000-0000-000000000000
        output: nir_band
        as: band_2
    args: {}
```

Each explicit reference specifies:

| Field | Required | Description |
|-------|----------|-------------|
| `block` | Yes | The invocation ID of the upstream block. |
| `output` | Yes | The name of the specific output on that upstream block. |
| `as` | No | The name of the input on the downstream block to connect to. If omitted, Spade matches by type. |

Explicit references give you full control over wiring. They are necessary when:

- An upstream block has **multiple outputs** and you need to pick a specific one
- A downstream block has **multiple inputs** and Spade cannot determine which output goes where by type alone
- You want to connect **two outputs from the same block** to different inputs on a downstream block (as in the example above)

## The resolution algorithm

When Spade resolves inputs for a block, it follows this procedure:

1. **Resolve explicit references first.** For each explicit reference (an object with `block` and `output`), Spade looks up the named output on the named upstream block and connects it to the specified downstream input (via `as`) or type-matches it to an unconnected input.

2. **Type-match remaining bare references.** For each bare reference (a plain invocation ID), Spade looks at the upstream block's outputs and the downstream block's unconnected inputs. It matches outputs to inputs by type and format. A `file` output with format `GeoTIFF` matches a `file` input with format `GeoTIFF`.

3. **Reject if ambiguous.** If a bare reference produces multiple possible matchings (e.g., the upstream block has two `file` outputs with the same format, and the downstream block has two `file` inputs with the same format), Spade cannot determine which output goes where. It rejects the pipeline with an error message explaining the ambiguity. Use explicit references to resolve the ambiguity.

4. **Reject if incomplete.** If, after processing all references, any downstream input remains unconnected and is not a parameter type (`string`, `number`, `boolean` provided via `args`), Spade rejects the pipeline with an error listing the unconnected inputs.

## When to use each form

### Use bare references when

The connection is straightforward:

- The upstream block has one output and the downstream block has one matching input
- Types and formats are distinct enough that there is no ambiguity

This is the most common case and keeps pipeline YAML concise.

### Use explicit references when

- The upstream block produces **multiple outputs** of the same type
- You need to route **specific outputs to specific inputs**
- You are connecting **multiple outputs from the same upstream block**
- Spade reports an ambiguity error with bare references

## Examples

### Simple one-to-one connection (bare reference)

```yaml
blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: data.download
    inputs: []
    args:
      url: "https://example.com/data.csv"

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: stats.summarize
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
    args:
      column: temperature
```

`data.download` produces one file output. `stats.summarize` expects one file input. Spade matches them automatically.

### Multiple outputs from one block (explicit references)

```yaml
blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: raster.split-bands
    inputs: []
    args:
      input_path: /data/multiband.tif

  # split-bands produces outputs: red, green, blue, nir (all type: file, format: GeoTIFF)

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.ndvi
    inputs:
      - block: 019cf4bc-1111-7000-0000-000000000000
        output: red
        as: red_band
      - block: 019cf4bc-1111-7000-0000-000000000000
        output: nir
        as: nir_band
    args: {}
```

Without explicit references, Spade would see four `file` outputs of format `GeoTIFF` and two `file` inputs of format `GeoTIFF`, and could not determine which output goes to which input. The explicit references make the wiring unambiguous.

### Multiple upstream blocks (mixed references)

```yaml
blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-105.5 40.0, -105.0 40.0, -105.0 40.5, -105.5 40.5, -105.5 40.0))"

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

Here `ml.classify` has two inputs: an image (`file`, format `GeoTIFF`) and a model (`file`, format `pickle`). The upstream blocks each produce one output of different formats, so Spade can match them by type and format without ambiguity. Bare references work fine.

## What happens with ambiguous connections

If Spade detects an ambiguity during input resolution, it reports a clear error:

```
Error: Ambiguous input resolution for block '019cf4bc-3333-7000-0000-000000000000' (raster.composite).
  Upstream block '019cf4bc-1111-7000-0000-000000000000' (raster.split-bands) has
  multiple outputs matching input 'band_1':
    - 'red' (file, GeoTIFF)
    - 'green' (file, GeoTIFF)
    - 'blue' (file, GeoTIFF)
    - 'nir' (file, GeoTIFF)
  Use explicit references to specify which output connects to which input.
```

The fix is to replace the bare reference with explicit references that name the exact outputs, as shown in the examples above.
