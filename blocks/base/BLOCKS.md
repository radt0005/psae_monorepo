# Base Block Collection — Block List

This document plans the blocks that make up the `base` collection: the core data-processing blocks for the Spade system. The collection is implemented in Rust (`Cargo.toml` at the repository root) and ships as a single binary with one subcommand per block.

The blocks are derived from `./SPECIFICATION.md` and grouped into four sections matching the spec: **tabular data**, **map**, **reduce**, and **utility**. Every tabular block reads CSV or Parquet and writes Parquet by default — Parquet is the preferred internal format because it is columnar, typed, and self-describing.

## Implementation Notes

- **Tabular engine:** [Polars](https://pola.rs/) is the recommended dependency. It handles CSV and Parquet natively, has a rich expression API for filters/aggregations, and is fast. [DataFusion](https://datafusion.apache.org/) is a credible alternative if SQL parsing is preferred over an expression DSL.
- **Format detection:** tabular blocks accept either CSV or Parquet by inspecting the file extension or magic bytes, so they can be wired into pipelines without an explicit conversion step.
- **List-typed parameters:** the manifest type system only supports `string`/`number`/`boolean` scalars. Where a block needs a list (e.g. column names, aggregation specs), the parameter is declared as `string` and parsed as JSON or comma-separated values inside the handler. This is called out in the relevant blocks below.
- **Dispatch:** add each new block to `src/lib.rs` (re-export) and `src/main.rs` (subcommand match), following the same pattern as the existing `map_files` block.

---

## 1. Tabular Data Blocks

### `base.filter_rows`
- **Kind:** `standard`
- **Description:** Filter rows of a table by a predicate expression (the SQL `WHERE` analogue).
- **Inputs:**
  - `table` — `file`, format `Parquet` (CSV also accepted) — the table to filter.
  - `expression` — `string` — a SQL `WHERE`-style predicate, e.g. `"age > 30 AND state = 'NY'"`. Parsed by the engine into a Polars expression / DataFusion predicate.
- **Outputs:**
  - `result` — `file`, format `Parquet` — the filtered table.

### `base.select_columns`
- **Kind:** `standard`
- **Description:** Project a subset of columns (the SQL `SELECT col1, col2` analogue). Optionally inverts to a "drop these columns" mode.
- **Inputs:**
  - `table` — `file`, format `Parquet` — the source table.
  - `columns` — `string` — comma-separated column names, e.g. `"id,name,score"`.
  - `mode` — `string` — `"keep"` (default) or `"drop"`.
- **Outputs:**
  - `result` — `file`, format `Parquet` — the projected table.

### `base.aggregate`
- **Kind:** `standard`
- **Description:** Compute aggregations over a table, optionally grouped. Supports mean, median, mode, sum, min, max, count, count_distinct, std, var, and configurable percentiles. With no grouping columns, returns a single-row table; with grouping columns, returns one row per group (this absorbs the "group by" use case from the spec).
- **Inputs:**
  - `table` — `file`, format `Parquet` — the source table.
  - `aggregations` — `string` — JSON list of aggregation specs, e.g.
    ```json
    [
      {"column": "score", "function": "mean", "as": "score_mean"},
      {"column": "score", "function": "percentile", "p": 0.95, "as": "score_p95"},
      {"column": "id", "function": "count", "as": "n"}
    ]
    ```
  - `group_by` — `string` — comma-separated column names to group on (empty string means no grouping).
- **Outputs:**
  - `result` — `file`, format `Parquet` — the aggregated table.

### `base.group_by`
- **Kind:** `standard`
- **Description:** Thin convenience wrapper around `aggregate` that takes the group columns as the primary parameter and an aggregation list. Kept as a distinct block because pipelines authored in the web UI read more naturally with an explicit "group by" step. Internally identical to `aggregate` with a non-empty `group_by`.
- **Inputs:**
  - `table` — `file`, format `Parquet`.
  - `group_columns` — `string` — comma-separated column names.
  - `aggregations` — `string` — same JSON format as `base.aggregate`.
- **Outputs:**
  - `result` — `file`, format `Parquet`.

> If we decide one block covers both cases cleanly, drop `base.group_by` and keep only `base.aggregate`. Listed separately here to surface the choice.

---

## 2. Map Blocks

### `base.map_files` *(already exists)*
- **Kind:** `map`
- **Description:** Enumerate the files of an input collection and emit one expansion item per file. The canonical "for each file in this directory" map.
- **Inputs:**
  - `source` — `collection` of `file` — the collection to fan out over.
- **Outputs:**
  - `manifest` — `expansion` — one item per file, with `key` set to the file stem and `path` pointing at the file under `inputs/source/`.

The current manifest at `blocks/map_files.yaml` is a stub (`inputs: {}`, `outputs: {}`); it should be filled in to match the schema above when implementation lands.

### `base.map_list`
- **Kind:** `map`
- **Description:** Fan out over a literal list of values supplied as a parameter (e.g. `["NY", "MI", "CA"]`, or a list of integer simulation seeds). Each value is materialised as a per-item JSON file under `outputs/manifest/items/<index>.json` so downstream blocks have a real file to consume; the expansion manifest's `path` points at that file and `key` is the value (stringified).
- **Inputs:**
  - `values` — `string` — JSON-encoded array of strings or numbers, e.g. `'["NY","MI","CA"]'` or `'[1,2,3,4,5]'`.
- **Outputs:**
  - `manifest` — `expansion` — one item per value.

### `base.map_range`
- **Kind:** `map`
- **Description:** Fan out over a numeric range (inclusive `start`, exclusive `end`, optional `step`). Useful for repeating a simulation N times in parallel without hand-writing the list. Each item is materialised the same way as `map_list`.
- **Inputs:**
  - `start` — `number`.
  - `end` — `number`.
  - `step` — `number` — defaults to `1` if omitted.
- **Outputs:**
  - `manifest` — `expansion`.

> If we want to keep the surface minimal, `map_range` can be omitted and users can call `map_list` with a generated array. Listed because "run a simulation many times" is one of the explicit motivating use cases in the spec.

---

## 3. Reduce Blocks

### `base.reduce_collection`
- **Kind:** `reduce`
- **Description:** The minimum-viable reducer. Gathers the outputs of N mapped invocations back into a single `collection` output, preserving filenames. This is the "do nothing clever, just regroup" reducer the spec calls for.
- **Inputs:**
  - `items` — `collection` of `file` — outputs of the upstream mapped block.
- **Outputs:**
  - `result` — `collection` of `file` — the same items, regrouped as a collection for downstream consumption.

### `base.reduce_stack`
- **Kind:** `reduce`
- **Description:** Concatenate a collection of tables row-wise (R's `rbind`, pandas' `concat(axis=0)`, SQL `UNION ALL`). All tables must share a compatible schema; mismatched columns can either be filled with nulls or rejected based on a `strict` flag.
- **Inputs:**
  - `tables` — `collection` of `file`, format `Parquet` (CSV accepted) — the tables to stack.
  - `strict` — `boolean` — when `true`, requires identical schemas; when `false` (default), unions schemas and fills missing columns with null.
- **Outputs:**
  - `result` — `file`, format `Parquet` — the stacked table.

### `base.reduce_join`
- **Kind:** `reduce`
- **Description:** Join a collection of tables on one or more key columns (the SQL `JOIN` analogue). Tables are joined sequentially in the collection's order, accumulating left-to-right.
- **Inputs:**
  - `tables` — `collection` of `file`, format `Parquet` — the tables to join.
  - `on` — `string` — comma-separated key column names.
  - `how` — `string` — one of `"inner"`, `"left"`, `"right"`, `"outer"`. Defaults to `"inner"`.
- **Outputs:**
  - `result` — `file`, format `Parquet` — the joined table.

---

## 4. Utility Blocks

### `base.csv_to_parquet`
- **Kind:** `standard`
- **Description:** Convert a CSV file to Parquet. Useful as an explicit conversion step at pipeline boundaries; tabular blocks accept CSV directly, but converting once up-front avoids re-parsing CSV at every step.
- **Inputs:**
  - `table` — `file`, format `CSV`.
  - `delimiter` — `string` — defaults to `","`.
  - `has_header` — `boolean` — defaults to `true`.
- **Outputs:**
  - `result` — `file`, format `Parquet`.

### `base.parquet_to_csv`
- **Kind:** `standard`
- **Description:** Convert a Parquet file to CSV. Useful at pipeline boundaries when downstream consumers (humans, spreadsheets, legacy tools) need CSV.
- **Inputs:**
  - `table` — `file`, format `Parquet`.
  - `delimiter` — `string` — defaults to `","`.
  - `include_header` — `boolean` — defaults to `true`.
- **Outputs:**
  - `result` — `file`, format `CSV`.

---

## Summary

| Block                    | Kind     | Section  |
| ------------------------ | -------- | -------- |
| `base.filter_rows`       | standard | Tabular  |
| `base.select_columns`    | standard | Tabular  |
| `base.aggregate`         | standard | Tabular  |
| `base.group_by`          | standard | Tabular  |
| `base.map_files`         | map      | Map      |
| `base.map_list`          | map      | Map      |
| `base.map_range`         | map      | Map      |
| `base.reduce_collection` | reduce   | Reduce   |
| `base.reduce_stack`      | reduce   | Reduce   |
| `base.reduce_join`       | reduce   | Reduce   |
| `base.csv_to_parquet`    | standard | Utility  |
| `base.parquet_to_csv`    | standard | Utility  |

Twelve blocks total, covering every system listed in `SPECIFICATION.md`. Three (`base.group_by`, `base.map_range`, `base.parquet_to_csv`) are flagged above as "include if useful, omit if you want a smaller surface" — strict adherence to the spec without these still yields nine blocks that satisfy every requirement.
