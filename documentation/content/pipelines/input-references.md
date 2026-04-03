+++
title = "Input References"
description = "How to wire block inputs using bare and explicit reference styles."
weight = 2
+++

When a block needs data from an upstream block, you list the upstream block's invocation ID in the `inputs` field. Spade supports two reference styles: **bare references** and **explicit references**. You can mix both styles in the same `inputs` list.

## Bare references

A bare reference is the simplest form. You provide only the invocation ID of the upstream block:

```yaml
inputs:
  - 019cf4bc-1111-7000-0000-000000000000
```

When Spade encounters a bare reference, it uses type matching to determine which output from the upstream block connects to which input on the current block. This works well when the connection is unambiguous -- for example, when the upstream block produces a single raster output and the current block expects a single raster input.

### Example: unambiguous bare reference

Consider two blocks:

- **`data.sentinel2`** produces one output: `image` (type: `file`, format: `GeoTIFF`)
- **`raster.reproject`** expects one file input: `raster` (type: `file`, format: `GeoTIFF`)

The pipeline:

```yaml
blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((-122.5 37.5, -122.0 37.5, -122.0 38.0, -122.5 38.0, -122.5 37.5))"

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.reproject
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
    args:
      target_crs: "EPSG:4326"
```

Spade sees that `data.sentinel2` produces a GeoTIFF output and `raster.reproject` expects a GeoTIFF input. There is exactly one way to match them, so the bare reference is sufficient.

## Explicit references

An explicit reference is an object with two keys:

| Key      | Description |
|----------|-------------|
| `block`  | The invocation ID of the upstream block |
| `output` | The name of the specific output to use |

```yaml
inputs:
  - block: 019cf4bc-1111-7000-0000-000000000000
    output: classified_raster
```

Explicit references are necessary when a bare reference would be ambiguous. This happens when the upstream block produces multiple outputs of the same type, or when there are multiple upstream blocks whose outputs could match the same input.

### Example: explicit reference needed

Consider a block called `raster.split-bands` that produces two outputs:

- `red` (type: `file`, format: `GeoTIFF`)
- `nir` (type: `file`, format: `GeoTIFF`)

And a downstream block `raster.band-ratio` that expects two inputs:

- `numerator` (type: `file`, format: `GeoTIFF`)
- `denominator` (type: `file`, format: `GeoTIFF`)

Using bare references here would be ambiguous -- Spade cannot determine which output (`red` or `nir`) maps to which input (`numerator` or `denominator`). You must use explicit references:

```yaml
blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: raster.split-bands
    inputs:
      - 019cf4bc-0000-7000-0000-000000000000
    args:
      red_band: 4
      nir_band: 8

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.band-ratio
    inputs:
      - block: 019cf4bc-1111-7000-0000-000000000000
        output: nir
      - block: 019cf4bc-1111-7000-0000-000000000000
        output: red
    args: {}
```

Here, the `nir` output is explicitly wired to the `numerator` input and the `red` output is wired to the `denominator` input. Spade matches explicit references to inputs by type after fixing the output -- since each explicit reference names exactly one output, there is only one type to match, and Spade assigns it to the compatible input.

## Mixed references

You can combine bare and explicit references in the same `inputs` list. This is useful when some connections are unambiguous and others are not.

```yaml
blocks:
  - id: 019cf4bc-3333-7000-0000-000000000000
    name: analysis.combine
    inputs:
      # Bare reference: the upstream block has one output that
      # matches one of this block's inputs by type.
      - 019cf4bc-1111-7000-0000-000000000000

      # Explicit reference: this upstream block has multiple
      # outputs, so we name the specific one we want.
      - block: 019cf4bc-2222-7000-0000-000000000000
        output: summary_stats
```

Spade resolves explicit references first, then resolves the remaining bare references against the unmatched inputs.

## Type-matching algorithm

When Spade processes a block's `inputs` list, it runs the following algorithm to wire upstream outputs to the current block's declared inputs:

**Step 1: Gather declared inputs.** Read the current block's manifest to find all declared inputs and their types. Separate file-type inputs (which come from upstream blocks) from scalar inputs (which come from `args`).

**Step 2: Resolve explicit references.** For each explicit reference in the `inputs` list:

   1. Look up the upstream block invocation by its `block` ID.
   2. Look up the named `output` in that upstream block's manifest.
   3. Find the declared input on the current block whose type is compatible with the output's type.
   4. If exactly one input matches, wire them together and mark both the output and the input as resolved.
   5. If no input matches, report a type-mismatch error.
   6. If multiple inputs match, report an ambiguity error (you need to restructure the pipeline or use additional explicit references).

**Step 3: Resolve bare references.** For each bare reference in the `inputs` list:

   1. Look up the upstream block invocation by its ID.
   2. Collect all of that block's outputs that have not yet been resolved.
   3. For each unresolved output, find all unresolved inputs on the current block with a compatible type.
   4. If there is exactly one way to pair all unresolved outputs to unresolved inputs, wire them and mark them as resolved.
   5. If multiple pairings are possible, report an ambiguity error and suggest using explicit references.
   6. If an output has no compatible unresolved input, report a type-mismatch error.

**Step 4: Verify completeness.** After resolving all references, check that every required file-type input on the current block has been wired to an upstream output. If any required input remains unwired, report a missing-input error.

### Walkthrough: unambiguous resolution

Suppose Block A produces:
- `image` (type: `file`, format: `GeoTIFF`)

And Block B expects:
- `raster` (type: `file`, format: `GeoTIFF`)
- `threshold` (type: `number`, from args)

Block B's pipeline entry:

```yaml
- id: 019cf4bc-2222-7000-0000-000000000000
  name: raster.classify
  inputs:
    - 019cf4bc-1111-7000-0000-000000000000
  args:
    threshold: 0.5
```

Resolution proceeds as:

1. Declared file-type inputs: `raster` (GeoTIFF). Scalar inputs: `threshold` (from args).
2. No explicit references.
3. Bare reference to Block A. Block A has one unresolved output: `image` (GeoTIFF). Block B has one unresolved file-type input: `raster` (GeoTIFF). Types are compatible. Exactly one pairing exists. Wire `image` to `raster`.
4. All required inputs are satisfied.

### Walkthrough: ambiguous resolution

Suppose Block A produces:
- `red` (type: `file`, format: `GeoTIFF`)
- `nir` (type: `file`, format: `GeoTIFF`)

And Block B expects:
- `band_a` (type: `file`, format: `GeoTIFF`)
- `band_b` (type: `file`, format: `GeoTIFF`)

Block B's pipeline entry using bare references:

```yaml
- id: 019cf4bc-2222-7000-0000-000000000000
  name: raster.difference
  inputs:
    - 019cf4bc-1111-7000-0000-000000000000
  args: {}
```

Resolution proceeds as:

1. Declared file-type inputs: `band_a` (GeoTIFF), `band_b` (GeoTIFF).
2. No explicit references.
3. Bare reference to Block A. Unresolved outputs: `red` (GeoTIFF), `nir` (GeoTIFF). Unresolved inputs: `band_a` (GeoTIFF), `band_b` (GeoTIFF). Both outputs are compatible with both inputs. Two possible pairings exist: (`red`->`band_a`, `nir`->`band_b`) or (`red`->`band_b`, `nir`->`band_a`).
4. **Ambiguity error.** Spade reports the conflict and suggests using explicit references.

The fix is to use explicit references:

```yaml
- id: 019cf4bc-2222-7000-0000-000000000000
  name: raster.difference
  inputs:
    - block: 019cf4bc-1111-7000-0000-000000000000
      output: nir
    - block: 019cf4bc-1111-7000-0000-000000000000
      output: red
  args: {}
```

## When to use each style

| Situation | Recommended style |
|-----------|-------------------|
| Upstream block has one output and downstream block has one matching input | Bare reference |
| Upstream block has multiple outputs of different types | Bare reference (types are still distinguishable) |
| Upstream block has multiple outputs of the same type | Explicit reference |
| Multiple upstream blocks produce outputs of the same type for a multi-input block | Explicit reference |
| You want maximum clarity regardless of ambiguity | Explicit reference |

As a general rule: start with bare references for simplicity. If `spade check` reports an ambiguity error, switch to explicit references for the affected connections.
