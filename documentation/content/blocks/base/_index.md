+++
title = "base"
description = "Core tabular data processing, map, and reduce blocks. Implemented in Rust."
weight = 1
sort_by = "weight"
insert_anchor_links = "right"
+++

The `base` collection is Spade's core block collection. It provides the primitives every pipeline is likely to need: tabular data transformations, map blocks for fan-out, reduce blocks for fan-in, and format conversion utilities.

The collection is implemented in Rust and ships as a single binary with one subcommand per block. Tabular blocks use [Polars](https://pola.rs/) and accept either CSV or Parquet input — Parquet is the preferred internal format because it is columnar, typed, and self-describing.

## Tabular blocks

These blocks operate on tables (CSV or Parquet files) and produce Parquet output by default.

| Block | Kind | Description |
|-------|------|-------------|
| [`base.filter_rows`](/blocks/base/filter_rows/) | standard | Filter rows by a SQL `WHERE`-style predicate |
| [`base.select_columns`](/blocks/base/select_columns/) | standard | Keep or drop a subset of columns |
| [`base.aggregate`](/blocks/base/aggregate/) | standard | Compute aggregations over a table, optionally grouped |
| [`base.group_by`](/blocks/base/group_by/) | standard | Group by columns and aggregate (convenience wrapper) |
| [`base.csv_to_parquet`](/blocks/base/csv_to_parquet/) | standard | Convert a CSV file to Parquet |
| [`base.parquet_to_csv`](/blocks/base/parquet_to_csv/) | standard | Convert a Parquet file to CSV |

## Map blocks

Map blocks enumerate items and emit an expansion manifest that fans downstream blocks out in parallel.

| Block | Kind | Description |
|-------|------|-------------|
| [`base.map_files`](/blocks/base/map_files/) | map | Fan out over every file in a collection |
| [`base.map_list`](/blocks/base/map_list/) | map | Fan out over a literal list of scalar values |
| [`base.map_range`](/blocks/base/map_range/) | map | Fan out over a numeric range |

## Reduce blocks

Reduce blocks collect mapped outputs back into a single result.

| Block | Kind | Description |
|-------|------|-------------|
| [`base.reduce_collection`](/blocks/base/reduce_collection/) | reduce | Gather mapped outputs back into a collection |
| [`base.reduce_stack`](/blocks/base/reduce_stack/) | reduce | Concatenate tables row-wise (`rbind` / `UNION ALL`) |
| [`base.reduce_join`](/blocks/base/reduce_join/) | reduce | Join tables on key columns (SQL `JOIN`) |

## Installation

```bash
spade install file:///path/to/blocks/base
```
