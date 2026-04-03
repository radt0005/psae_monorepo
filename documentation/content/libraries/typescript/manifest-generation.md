+++
title = "Manifest Generation"
description = "Auto-generating block manifests from TypeScript metadata."
weight = 4
+++

The TypeScript library can generate block manifest data from the `SpadeMetadata` attached to your handler. This keeps your manifest and implementation in sync -- you define the types once in your handler metadata, and the library produces the corresponding YAML structure.

## The `build()` function

`build(fn)` reads the metadata from a handler function and returns a plain object that matches the block manifest schema:

```typescript
import { build, spadeBlock, RasterFile, JsonFile } from "spade";

const handler = spadeBlock({
  inputs: {
    source: RasterFile,
    resolution: "number",
  },
  output: RasterFile,
  description: "Reprojects a raster to a target resolution",
})(function handler({ source, resolution }) {
  return new RasterFile("result.tif");
});

const manifest = build(handler);
console.log(JSON.stringify(manifest, null, 2));
```

This produces:

```json
{
  "description": "Reprojects a raster to a target resolution",
  "inputs": {
    "source": {
      "type": "file",
      "format": "GeoTIFF"
    },
    "resolution": {
      "type": "number"
    }
  },
  "outputs": {
    "raster": {
      "type": "file",
      "format": "GeoTIFF"
    }
  }
}
```

## How type mapping works

The library maps each TypeScript class or scalar literal to its manifest representation:

| TypeScript type | Manifest entry |
|----------------|----------------|
| `File` | `{ type: "file" }` |
| `RasterFile` | `{ type: "file", format: "GeoTIFF" }` |
| `VectorFile` | `{ type: "file", format: "GeoJSON" }` |
| `TabularFile` | `{ type: "file", format: "CSV" }` |
| `JsonFile` | `{ type: "json" }` |
| `Directory` | `{ type: "directory" }` |
| `FileCollection` | `{ type: "collection", item_type: "file" }` |
| `RasterFileCollection` | `{ type: "collection", item_type: "file", format: "GeoTIFF" }` |
| `VectorFileCollection` | `{ type: "collection", item_type: "file", format: "GeoJSON" }` |
| `TabularFileCollection` | `{ type: "collection", item_type: "file", format: "CSV" }` |
| `"string"` | `{ type: "string" }` |
| `"number"` | `{ type: "number" }` |
| `"boolean"` | `{ type: "boolean" }` |

## Output naming

When a single output type is declared via the `output` field in metadata, the library assigns a default output name based on the type:

| Type | Default output name |
|------|---------------------|
| `File` | `file` |
| `RasterFile` | `raster` |
| `VectorFile` | `vector` |
| `TabularFile` | `tabular` |
| `JsonFile` | `json` |
| `Directory` | `directory` |
| `FileCollection` | `files` |
| `RasterFileCollection` | `rasters` |
| `VectorFileCollection` | `vectors` |
| `TabularFileCollection` | `tables` |

## Description

If the `SpadeMetadata` includes a `description` field, it appears as a top-level key in the generated manifest:

```typescript
const handler = spadeBlock({
  inputs: { source: RasterFile },
  output: RasterFile,
  description: "Applies a smoothing filter to a raster",
})(function handler({ source }) {
  return new RasterFile("result.tif");
});

const manifest = build(handler);
// manifest.description === "Applies a smoothing filter to a raster"
```

## Generating YAML

The `build()` function returns a plain JavaScript object. To produce YAML for a `block.yaml` file, serialize it with a YAML library:

```typescript
import yaml from "js-yaml";
import { build } from "spade";

const manifest = build(handler);
const yamlStr = yaml.dump(manifest);
console.log(yamlStr);
```

Output:

```yaml
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

## Handlers without metadata

If `build()` is called on a handler that has no attached metadata, it returns a minimal object:

```typescript
function plainHandler(args: Record<string, unknown>) {
  // no metadata attached
}

const manifest = build(plainHandler);
// { inputs: {}, outputs: {} }
```

## Multiple inputs example

```typescript
const handler = spadeBlock({
  inputs: {
    raster: RasterFile,
    boundary: VectorFile,
    tiles: RasterFileCollection,
    buffer: "number",
    normalize: "boolean",
  },
  output: RasterFile,
  description: "Clips and normalizes a raster",
})(function handler(args) {
  return new RasterFile("result.tif");
});

const manifest = build(handler);
```

This generates manifest entries for all five inputs with their respective types and formats.
