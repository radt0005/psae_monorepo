+++
title = "Multi-Language Blocks"
weight = 1
description = "Write processing blocks in Python, R, Go, Rust, or TypeScript — Spade handles the rest."
template = "features/page.html"
+++

Spade blocks can be written in any of five supported languages, giving your team the freedom to use the best tool for each job.

## Supported Languages

- **Python** — Executed via `uv run`, with full access to the Python ecosystem
- **R** — Executed via `Rscript`, ideal for statistical analysis and data visualization
- **Go** — Compiled to a single binary with subcommand support
- **Rust** — Compiled to a single binary for maximum performance
- **TypeScript** — Bundled and executed via Bun for fast JavaScript-based processing

## The Handler Pattern

Every block follows the same simple pattern regardless of language: define a handler function that accepts typed inputs and returns typed outputs. The Spade library for each language handles parameter loading, input reading, and output writing automatically.

```python
from spade import run, RasterFile

def handler(source: RasterFile) -> RasterFile:
    # Your processing logic here
    return RasterFile(path=result)

if __name__ == "__main__":
    run(handler)
```

## Type-Safe I/O

Spade's type system includes:

- **RasterFile** — Georeferenced raster data (GeoTIFF, etc.)
- **VectorFile** — Vector geometry data (GeoJSON, Shapefile, etc.)
- **TabularFile** — Tabular data (CSV, Parquet, etc.)
- **Collections** — Variable-length ordered sets of any file type
- **Scalars** — JSON-serializable values passed via parameters

The library validates types at runtime, catching errors early before they cascade through your pipeline.
