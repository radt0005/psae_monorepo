+++
title = "Quickstart"
description = "Create your first TypeScript block step by step."
weight = 1
+++

This guide walks you through creating a Spade block in TypeScript using the Bun runtime. By the end you will have a working block that reads a raster file, applies a buffer parameter, and writes the result.

## Prerequisites

- **Bun** 1.0 or later
- **TypeScript 5.0** or later
- The **Spade CLI** installed ([Installation guide](/getting-started/installation/))

## Step 1: Create a block collection

```bash
mkdir raster-tools && cd raster-tools
spade init --language typescript
```

This scaffolds the project:

```
raster-tools/
  package.json
  src/
  blocks/
```

Install the Spade library:

```bash
bun add spade
```

## Step 2: Add a block

```bash
spade add reproject
```

This creates:

1. **`blocks/reproject.yaml`** -- the block manifest
2. **`src/reproject.ts`** -- the handler entrypoint

## Step 3: Define the manifest

Edit `blocks/reproject.yaml`:

```yaml
id: raster-tools.reproject
version: 0.1.0
kind: standard
network: false
description: Reprojects a raster to a target resolution

inputs:
  source:
    type: file
    format: GeoTIFF
  resolution:
    type: number

outputs:
  raster:
    type: file
    format: GeoTIFF
```

## Step 4: Write the handler

Edit `src/reproject.ts`:

```typescript
import {
  run,
  spadeBlock,
  RasterFile,
} from "spade";

const handler = spadeBlock({
  inputs: {
    source: RasterFile,
    resolution: "number",
  },
  output: RasterFile,
  description: "Reprojects a raster to a target resolution",
})(function handler({ source, resolution }: { source: RasterFile; resolution: number }) {
  console.log(`Reprojecting ${source.path} at resolution ${resolution}`);

  // Your raster processing logic here
  const outputPath = "outputs/raster/reprojected.tif";

  return new RasterFile(outputPath);
});

await run(handler);
```

Key concepts:

- **`spadeBlock`** is a decorator factory that attaches type metadata to your handler via a `WeakMap`. The metadata tells `run()` how to load inputs and tells `build()` how to generate the manifest.
- The handler receives a single object. File inputs arrive as typed class instances (e.g., `RasterFile` with a `.path` property). Scalar parameters like `resolution` arrive as plain values.
- **`run(handler)`** is the entry point, and it's `async` -- call it with `await`. It reads `params.yaml` for scalar parameters, scans `inputs/` for file inputs, merges everything into a single argument object, calls your handler (awaiting it if it returns a `Promise`), and writes outputs.

## Step 5: Validate and install

```bash
spade check
spade install file://.
```

## Step 6: Use in a pipeline

```yaml
blocks:
  - id: "@reproject"
    name: raster-tools.reproject
    inputs: []
    args:
      resolution: 10
```

## Alternative: setMetadata

If you prefer not to use the decorator pattern, you can attach metadata with `setMetadata` directly:

```typescript
import { run, setMetadata, RasterFile } from "spade";

function handler({ source, resolution }: { source: RasterFile; resolution: number }) {
  return new RasterFile("outputs/raster/reprojected.tif");
}

setMetadata(handler, {
  inputs: { source: RasterFile, resolution: "number" },
  output: RasterFile,
});

await run(handler);
```

Both approaches store the same metadata and produce identical behavior at runtime.

## Next steps

- [Types](/libraries/typescript/types/) -- all available Spade types
- [Handler Functions](/libraries/typescript/handlers/) -- handler patterns, async `run()`, secrets, and multiple outputs
- [Manifest Generation](/libraries/typescript/manifest-generation/) -- auto-generating `block.yaml` from metadata
- [Examples](/libraries/typescript/examples/) -- complete worked examples
