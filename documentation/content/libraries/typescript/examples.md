+++
title = "Examples"
description = "Complete worked examples of TypeScript blocks."
weight = 5
+++

These examples show complete, working TypeScript blocks covering common patterns. Each example includes the block manifest, the handler implementation, and a description of the directory layout at runtime.

## Example 1: Raster reprojection

A block that reads a raster file and a resolution parameter, reprojects the raster, and writes the result.

### Manifest (`blocks/reproject.yaml`)

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

### Handler (`src/reproject.ts`)

```typescript
import { run, spadeBlock, RasterFile } from "spade";
import { execSync } from "node:child_process";

const handler = spadeBlock({
  inputs: {
    source: RasterFile,
    resolution: "number",
  },
  output: RasterFile,
  description: "Reprojects a raster to a target resolution",
})(function handler({
  source,
  resolution,
}: {
  source: RasterFile;
  resolution: number;
}) {
  const outputPath = "reprojected.tif";

  execSync(
    `gdalwarp -tr ${resolution} ${resolution} "${source.path}" "${outputPath}"`
  );

  return new RasterFile(outputPath);
});

run(handler);
```

### Runtime directory layout

```
inputs/
  source/
    original.tif
params.yaml          # resolution: 10
outputs/
  raster/
    reprojected.tif
```

---

## Example 2: CSV data analysis

A block that reads a CSV file, computes summary statistics for a specified column, and writes the results as JSON.

### Manifest (`blocks/summarize.yaml`)

```yaml
id: data-tools.summarize
version: 0.1.0
kind: standard
network: false
description: Computes summary statistics for a CSV column

inputs:
  data:
    type: file
    format: CSV
  column:
    type: string

outputs:
  stats:
    type: json
```

### Handler (`src/summarize.ts`)

```typescript
import { run, spadeBlock, TabularFile, JsonFile } from "spade";
import { readFileSync, writeFileSync } from "node:fs";

const handler = spadeBlock({
  inputs: {
    data: TabularFile,
    column: "string",
  },
  output: JsonFile,
  description: "Computes summary statistics for a CSV column",
})(function handler({
  data,
  column,
}: {
  data: TabularFile;
  column: string;
}) {
  const content = readFileSync(data.path, "utf-8");
  const lines = content.trim().split("\n");
  const headers = lines[0].split(",").map((h) => h.trim());
  const colIndex = headers.indexOf(column);

  if (colIndex === -1) {
    throw new Error(`Column '${column}' not found in CSV headers`);
  }

  const values: number[] = [];
  for (let i = 1; i < lines.length; i++) {
    const val = parseFloat(lines[i].split(",")[colIndex]);
    if (!isNaN(val)) {
      values.push(val);
    }
  }

  const stats = {
    column,
    count: values.length,
    mean: values.reduce((a, b) => a + b, 0) / values.length,
    min: Math.min(...values),
    max: Math.max(...values),
  };

  const outputPath = "summary.json";
  writeFileSync(outputPath, JSON.stringify(stats, null, 2));

  return new JsonFile(outputPath);
});

run(handler);
```

### Runtime directory layout

```
inputs/
  data/
    measurements.csv
params.yaml          # column: temperature
outputs/
  stats/
    summary.json
```

---

## Example 3: Map block -- batch raster processing

A map block processes each element of a collection independently. The Spade runtime invokes the handler once per input item.

### Manifest (`blocks/normalize.yaml`)

```yaml
id: raster-tools.normalize
version: 0.1.0
kind: map
network: false
description: Normalizes a raster file to 0-1 range

inputs:
  raster:
    type: file
    format: GeoTIFF

outputs:
  raster:
    type: file
    format: GeoTIFF
```

### Handler (`src/normalize.ts`)

```typescript
import { run, spadeBlock, RasterFile } from "spade";
import { execSync } from "node:child_process";
import { basename } from "node:path";

const handler = spadeBlock({
  inputs: {
    raster: RasterFile,
  },
  output: RasterFile,
  description: "Normalizes a raster file to 0-1 range",
})(function handler({ raster }: { raster: RasterFile }) {
  const inputName = basename(raster.path, ".tif");
  const outputPath = `${inputName}_normalized.tif`;

  // Use gdal_calc to normalize pixel values to 0-1
  execSync(
    `gdal_calc.py -A "${raster.path}" --outfile="${outputPath}" ` +
      `--calc="(A - A.min()) / (A.max() - A.min())" --type=Float32`
  );

  return new RasterFile(outputPath);
});

run(handler);
```

When used in a pipeline with a collection of rasters, the Spade runtime calls this handler once per raster file. Each invocation sees a single raster in `inputs/raster/`.

### Runtime directory layout (per invocation)

```
inputs/
  raster/
    tile_001.tif
outputs/
  raster/
    tile_001_normalized.tif
```

---

## Example 4: Multiple outputs

A handler that produces both a processed raster and a JSON statistics file.

### Handler (`src/analyze.ts`)

```typescript
import { run, spadeBlock, RasterFile, JsonFile } from "spade";
import { writeFileSync } from "node:fs";

const handler = spadeBlock({
  inputs: {
    source: RasterFile,
    threshold: "number",
  },
})(function handler({
  source,
  threshold,
}: {
  source: RasterFile;
  threshold: number;
}) {
  // Processing logic produces two outputs
  const rasterOut = "classified.tif";
  const statsOut = "classification_stats.json";

  // ... processing ...

  writeFileSync(
    statsOut,
    JSON.stringify({ threshold, classified_pixels: 42000 }, null, 2)
  );

  return {
    raster: new RasterFile(rasterOut),
    stats: new JsonFile(statsOut),
  };
});

run(handler);
```

### Runtime directory layout

```
inputs/
  source/
    input.tif
params.yaml          # threshold: 0.5
outputs/
  raster/
    classified.tif
  stats/
    classification_stats.json
```

The returned object keys (`raster`, `stats`) become the subdirectory names under `outputs/`.
