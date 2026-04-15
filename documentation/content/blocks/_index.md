+++
title = "Block Catalog"
description = "Reference for all built-in blocks: base data processing, data import, and GDAL geospatial operations."
weight = 6
sort_by = "weight"
insert_anchor_links = "right"
+++

Spade ships with three built-in block collections that cover most common data-processing needs. Each collection is a self-contained bundle of blocks you can install with `spade install` and reference from your pipelines.

| Collection | Blocks | Language | Purpose |
|------------|:-----:|----------|---------|
| [`base`](/blocks/base/) | 12 | Rust | Core tabular data processing, map/reduce primitives |
| [`data`](/blocks/data/) | 17 | Rust | Data import from remote storage and public datasets |
| [`gdal`](/blocks/gdal/) | 46 | Python | Raster and vector operations wrapping the GDAL library |

## Block naming

Every block is identified by `<collection>.<name>`, for example `base.filter_rows` or `gdal.warp`. When you reference a block in a pipeline, you use this fully-qualified name.

## Finding blocks

Use the search box at the top of this site to find a specific block. Each block has its own page documenting its inputs, outputs, and usage.

## Block kinds

Blocks come in three kinds that reflect their role in a pipeline:

- **`standard`** — a single invocation that consumes inputs and produces outputs.
- **`map`** — a fan-out block that emits an expansion manifest so downstream blocks run once per item.
- **`reduce`** — a fan-in block that consumes a collection produced by mapped blocks and emits a single consolidated output.

See [Core Concepts: Map/Reduce](/concepts/map-reduce/) for a full explanation of this pattern.

## Installing the built-in collections

Each collection is installed from its source directory or git URL:

```bash
spade install file:///path/to/blocks/base
spade install file:///path/to/blocks/data
spade install file:///path/to/blocks/gdal
```

After installation, blocks are registered in `~/.spade/registry.db` and can be referenced by any pipeline.
