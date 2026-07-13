+++
title = "Handler Functions"
description = "Writing TypeScript handlers with decorators."
weight = 3
+++

A handler is a function that receives typed inputs and parameters as a single argument object and returns one or more typed outputs. The Spade library calls your handler after loading all inputs and parameters.

## Basic handler pattern

Every handler follows the same shape:

```typescript
import { run, spadeBlock, RasterFile } from "spade";

const handler = spadeBlock({
  inputs: {
    source: RasterFile,
    resolution: "number",
  },
  output: RasterFile,
})(function handler({ source, resolution }: { source: RasterFile; resolution: number }) {
  // Process the input
  const outputPath = "outputs/raster/result.tif";
  // ... processing logic ...
  return new RasterFile(outputPath);
});

await run(handler);
```

## Attaching metadata

There are two ways to attach type metadata to a handler.

### The `spadeBlock` decorator

`spadeBlock` is a decorator factory. It accepts a `SpadeMetadata` object and returns a function wrapper that stores the metadata via a `WeakMap`:

```typescript
import { spadeBlock, RasterFile, VectorFile } from "spade";

const handler = spadeBlock({
  inputs: {
    raster: RasterFile,
    boundary: VectorFile,
    buffer: "number",
  },
  output: RasterFile,
  description: "Clips a raster to a vector boundary",
})(function handler({ raster, boundary, buffer }) {
  // ...
  return new RasterFile("outputs/raster/clipped.tif");
});
```

### The `setMetadata` function

For cases where you want to define the handler first and attach metadata separately:

```typescript
import { setMetadata, RasterFile, VectorFile } from "spade";

function handler({ raster, boundary, buffer }) {
  return new RasterFile("outputs/raster/clipped.tif");
}

setMetadata(handler, {
  inputs: {
    raster: RasterFile,
    boundary: VectorFile,
    buffer: "number",
  },
  output: RasterFile,
});
```

Both approaches produce identical runtime behavior.

## The `SpadeMetadata` interface

```typescript
interface SpadeMetadata {
  inputs: Record<string, SpadeTypeClass>;
  output?: SpadeTypeClass;
  description?: string;
}
```

- **`inputs`** -- maps parameter names to their types. File types use class constructors (`RasterFile`, `VectorFile`, etc.). Scalar types use string literals (`"string"`, `"number"`, `"boolean"`).
- **`output`** -- the return type. Optional if the handler returns an object for multiple outputs.
- **`description`** -- an optional description used during manifest generation.

## Receiving arguments

The handler receives a single object containing all inputs and parameters merged together. File inputs are class instances; scalar parameters are plain values:

```typescript
function handler({
  source,      // RasterFile instance (source.path is the file path)
  boundary,    // VectorFile instance
  resolution,  // number (from params.yaml)
  method,      // string (from params.yaml)
}: {
  source: RasterFile;
  boundary: VectorFile;
  resolution: number;
  method: string;
}) {
  console.log(source.path);     // "inputs/source/data.tif"
  console.log(boundary.path);   // "inputs/boundary/area.geojson"
  console.log(resolution);       // 10
  console.log(method);           // "bilinear"
}
```

## Secrets

Secrets are not merged into the handler's argument object the way inputs and parameters are -- request them explicitly by calling `getSecret`:

```typescript
function getSecret(name: string): string
```

`getSecret` reads a secret the pipeline bound to `name` via the block's `secrets:` field in the pipeline file. If `name` was never declared for this block, or the runtime failed to resolve it, `getSecret` throws -- a declared-but-unresolvable secret is a real error, not a silent empty string.

```typescript
import { run, spadeBlock, getSecret, TabularFile, JsonFile } from "spade";

const handler = spadeBlock({
  inputs: { data: TabularFile },
  output: JsonFile,
})(async function handler({ data }: { data: TabularFile }) {
  const connectionString = getSecret("db");

  // ... connect using connectionString and process data ...

  return new JsonFile("result.json");
});

await run(handler);
```

## Single output

When your handler returns a single typed value, the library writes it to the appropriate `outputs/` subdirectory. The output directory name comes from the manifest (if it declares exactly one output) or from the type's default name:

```typescript
// Returns a single RasterFile
// Written to outputs/raster/ (or the name declared in the manifest)
return new RasterFile("result.tif");
```

## Multiple outputs

Return a plain object where each key is an output name and each value is a typed instance:

```typescript
import { run, spadeBlock, RasterFile, JsonFile } from "spade";

const handler = spadeBlock({
  inputs: {
    source: RasterFile,
  },
})(function handler({ source }: { source: RasterFile }) {
  // Process and produce multiple outputs
  return {
    raster: new RasterFile("processed.tif"),
    stats: new JsonFile("statistics.json"),
  };
});

await run(handler);
```

Each key in the returned object becomes a subdirectory under `outputs/`:

```
outputs/
  raster/
    processed.tif
  stats/
    statistics.json
```

## No output

If your handler performs a side effect (e.g., logging, validation) and produces no output, return `null` or `undefined`:

```typescript
const handler = spadeBlock({
  inputs: { source: RasterFile },
})(function handler({ source }: { source: RasterFile }) {
  console.log(`Validated: ${source.path}`);
  return null;
});
```

## How `run()` works

`run()` is an `async` function -- call it with `await`. When you call `await run(handler)`:

1. **Reads metadata** -- retrieves the `SpadeMetadata` stored via `spadeBlock` or `setMetadata`
2. **Loads parameters** -- reads `params.yaml` and parses scalar values
3. **Scans inputs** -- walks `inputs/` subdirectories and constructs typed instances using the metadata's type hints
4. **Merges arguments** -- combines parameters and inputs into a single object (via `buildFunctionArgs`)
5. **Filters arguments** -- if metadata is present, only declared input names are passed through
6. **Calls handler** -- invokes your function with the merged argument object, awaiting the result whether the handler is synchronous or returns a `Promise`
7. **Writes outputs** -- inspects the resolved return value and copies files to `outputs/`

## Async `run()`

Unlike the Python or R libraries, `run()` in TypeScript is an `async` function. This lets your handler perform asynchronous work -- reading files, awaiting network calls if `network: true` is set in the block manifest, etc. -- and lets the runtime await your handler's result whether it's synchronous or returns a `Promise`.

A synchronous handler needs no changes -- `run()` simply awaits it like any other value:

```typescript
const handler = spadeBlock({
  inputs: { source: RasterFile },
  output: RasterFile,
})(function handler({ source }: { source: RasterFile }) {
  return new RasterFile("result.tif");
});

await run(handler);
```

An `async` handler that awaits I/O works the same way -- declare it `async` and return a `Promise`:

```typescript
const handler = spadeBlock({
  inputs: { source: RasterFile },
  output: RasterFile,
})(async function handler({ source }: { source: RasterFile }) {
  const response = await fetch("https://example.com/metadata.json");
  const metadata = await response.json();

  // ... use metadata while processing source ...

  return new RasterFile("result.tif");
});

await run(handler);
```

## Error handling

Errors thrown in your handler propagate to `run()`, which prints them to stderr. Use standard JavaScript/TypeScript error handling:

```typescript
function handler({ source }: { source: RasterFile }) {
  if (!source.path.endsWith(".tif")) {
    throw new Error("Expected a GeoTIFF file");
  }
  // ...
}
```
