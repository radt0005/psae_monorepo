+++
title = "Examples"
description = "Complete worked examples of Rust blocks."
weight = 5
+++

These examples show complete, working Rust blocks covering common patterns. Each example includes the block manifest, the handler implementation, and the directory layout at runtime.

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

### Handler (`src/main.rs`)

```rust
use spade::{run, Args, RasterFile};
use std::process::Command;

fn handler(args: Args) -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
    let source: RasterFile = args.input("source")?;
    let resolution: f64 = args.param("resolution")?;

    let output_path = "reprojected.tif";
    let status = Command::new("gdalwarp")
        .args([
            "-tr",
            &resolution.to_string(),
            &resolution.to_string(),
            &source.path,
            output_path,
        ])
        .status()?;

    if !status.success() {
        return Err("gdalwarp failed".into());
    }

    Ok(RasterFile::new(output_path))
}

fn main() {
    run(handler);
}
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

### Handler (`src/main.rs`)

```rust
use spade::{run, Args, TabularFile, JsonFile};
use std::fs;

fn handler(args: Args) -> Result<JsonFile, Box<dyn std::error::Error + Send + Sync>> {
    let data: TabularFile = args.input("data")?;
    let column: String = args.param("column")?;

    // Read CSV
    let content = fs::read_to_string(&data.path)?;
    let mut lines = content.lines();

    let headers: Vec<&str> = lines
        .next()
        .ok_or("empty CSV")?
        .split(',')
        .map(|h| h.trim())
        .collect();

    let col_idx = headers
        .iter()
        .position(|&h| h == column)
        .ok_or_else(|| format!("column '{}' not found", column))?;

    let mut values: Vec<f64> = Vec::new();
    for line in lines {
        let fields: Vec<&str> = line.split(',').collect();
        if let Some(field) = fields.get(col_idx) {
            if let Ok(v) = field.trim().parse::<f64>() {
                values.push(v);
            }
        }
    }

    let count = values.len();
    let sum: f64 = values.iter().sum();
    let min = values.iter().cloned().fold(f64::INFINITY, f64::min);
    let max = values.iter().cloned().fold(f64::NEG_INFINITY, f64::max);

    let stats = serde_json::json!({
        "column": column,
        "count": count,
        "mean": sum / count as f64,
        "min": min,
        "max": max,
    });

    let output_path = "summary.json";
    fs::write(output_path, serde_json::to_string_pretty(&stats)?)?;

    Ok(JsonFile::new(output_path))
}

fn main() {
    run(handler);
}
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

### Handler (`src/main.rs`)

```rust
use spade::{run, Args, RasterFile};
use std::path::Path;
use std::process::Command;

fn handler(args: Args) -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
    let raster: RasterFile = args.input("raster")?;

    let input_path = Path::new(&raster.path);
    let stem = input_path
        .file_stem()
        .and_then(|s| s.to_str())
        .unwrap_or("output");
    let output_path = format!("{}_normalized.tif", stem);

    let status = Command::new("gdal_calc.py")
        .args([
            "-A",
            &raster.path,
            &format!("--outfile={}", output_path),
            "--calc=(A - A.min()) / (A.max() - A.min())",
            "--type=Float32",
        ])
        .status()?;

    if !status.success() {
        return Err("gdal_calc failed".into());
    }

    Ok(RasterFile::new(output_path))
}

fn main() {
    run(handler);
}
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

A handler that produces both a processed raster and a JSON statistics file using the `Outputs` collection.

### Handler (`src/main.rs`)

```rust
use spade::{run, Args, Outputs, RasterFile, JsonFile};
use std::fs;

fn handler(args: Args) -> Result<Outputs, Box<dyn std::error::Error + Send + Sync>> {
    let source: RasterFile = args.input("source")?;
    let threshold: f64 = args.param("threshold")?;

    // Processing logic...
    let raster_out = "classified.tif";
    // ... write classified raster ...

    let stats_out = "classification_stats.json";
    let stats = serde_json::json!({
        "threshold": threshold,
        "classified_pixels": 42000,
    });
    fs::write(stats_out, serde_json::to_string_pretty(&stats)?)?;

    let mut outputs = Outputs::new();
    outputs.add("raster", RasterFile::new(raster_out));
    outputs.add("stats", JsonFile::new(stats_out));

    Ok(outputs)
}

fn main() {
    run(handler);
}
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

The names passed to `outputs.add()` become the subdirectory names under `outputs/`.
