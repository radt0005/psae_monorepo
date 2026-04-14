# Base Block Collection — Implementation Plan

This is a checklist for implementing the twelve blocks defined in `./BLOCKS.md` in Rust. The collection lives at `blocks/base/`, uses the `spade` runtime library at `../../libs/rust/`, and compiles to a single binary (`base`) that dispatches one subcommand per block.

Work proceeds in **phases**. Each phase leaves the crate in a compiling, testable state so you can ship partial progress.

---

## Phase 0 — Project scaffolding and shared infrastructure

Foundational work that every block depends on. Do this first; everything else branches from here.

### 0.1 Cargo setup

- [DONE] Open `Cargo.toml` and set `edition = "2024"` to match the `spade` runtime.
- [DONE] Add a `[dependencies]` section with:
  - [DONE] `spade = { path = "../../libs/rust" }` — the runtime library.
  - [DONE] `polars = { version = "0.46", features = ["lazy", "parquet", "csv", "json", "dtype-full", "strings", "temporal", "dynamic_group_by", "cross_join", "semi_anti_join", "is_in", "rows"] }` — tabular engine.
  - [DONE] `polars-sql = "0.46"` — SQL predicate parser for `filter_rows`.
  - [DONE] `serde = { version = "1", features = ["derive"] }`
  - [DONE] `serde_json = "1"` — parsing list parameters (JSON-encoded strings).
  - [DONE] `serde_yaml = "0.9"` — writing expansion manifests.
  - [DONE] `thiserror = "2"` — collection-local error types.
  - [DONE] `anyhow = "1"` — handler-level error convenience.
- [DONE] Add a `[dev-dependencies]` section with `tempfile = "3"` and `assert_fs = "1"`.
- [DONE] Add `[[bin]] name = "base"` with `path = "src/main.rs"` so the binary name matches the collection name.
- [DONE] Run `cargo check` and confirm a clean build before touching any block code.

### 0.2 Dispatcher wiring (`src/main.rs` and `src/lib.rs`)

- [DONE] Replace the `"Hello, world!"` stub in `src/main.rs` with a dispatcher that reads `std::env::args().nth(1)` and matches on the subcommand name.
- [DONE] Each arm calls `<block_module>::entry()`. Unknown subcommands print a usage message listing all blocks and exit with code 2.
- [DONE] In `src/lib.rs`, `pub mod` every block module and re-export each module's public surface. This keeps the crate usable as a library for integration tests without going through the binary.
- [DONE] Add one `pub mod common;` declaration; Phase 0.3 fills it.

### 0.3 Shared helpers (`src/common/mod.rs`)

Several blocks share utilities. Extract these once rather than duplicating them per block.

- [DONE] `common::table` submodule:
  - [DONE] `pub fn read_table(path: &str) -> Result<DataFrame>` — dispatches on extension (`.csv` → `CsvReader`, `.parquet` → `ParquetReader`). Return a typed `BaseError::UnsupportedFormat` for anything else.
  - [DONE] `pub fn read_table_lazy(path: &str) -> Result<LazyFrame>` — same dispatch but returns `LazyFrame` for pipelined operations.
  - [DONE] `pub fn write_parquet(df: &mut DataFrame, path: &str) -> Result<()>` — uses `ParquetWriter` with Snappy compression.
  - [DONE] `pub fn write_csv(df: &mut DataFrame, path: &str, delimiter: u8, has_header: bool) -> Result<()>`.
- [DONE] `common::params` submodule:
  - [DONE] `pub fn parse_csv_list(s: &str) -> Vec<String>` — splits on `,`, trims, drops empties. Used by every block that takes `columns`, `group_by`, `on`, etc.
  - [DONE] `pub fn parse_json_list<T: DeserializeOwned>(s: &str) -> Result<Vec<T>>` — used by `aggregate`, `map_list`.
- [DONE] `common::error` submodule:
  - [DONE] `BaseError` enum via `thiserror` with variants: `UnsupportedFormat(String)`, `BadExpression(String)`, `SchemaMismatch(String)`, `InvalidAggregation(String)`, `EmptyCollection`, `Polars(#[from] polars::error::PolarsError)`, `Io(#[from] std::io::Error)`, `Json(#[from] serde_json::Error)`, `Yaml(#[from] serde_yaml::Error)`.
  - [DONE] Provide `pub type Result<T> = std::result::Result<T, BaseError>`.
- [DONE] Unit tests for every helper in `common/mod.rs` (happy path + at least one error path each).

### 0.4 Block skeleton macro

Every block module has the same shape: a `pub fn entry()` that calls `spade::run(|args| ...)`. To avoid copy-paste drift, add a tiny helper in `common/mod.rs`:

- [DONE] `pub fn handler_entry<F, T>(f: F)` where `F: FnOnce(spade::Args) -> Result<T>` and `T: spade::IntoOutput`. It wraps `f` so block modules reduce to one function each.
- [DONE] Document the convention in a top comment of `common/mod.rs`.

### 0.5 Test harness conventions

- [DONE] Create `tests/` at the crate root for integration tests. Each block gets one integration test file (e.g. `tests/filter_rows.rs`).
- [DONE] Write a shared helper in `tests/common/mod.rs` that builds a working directory with `inputs/`, `outputs/`, `params.yaml`, and invokes the block handler directly (not via the binary) using the block module's `entry_with_args` variant (see Phase 0.6).
- [DONE] Each block test:
  1. Creates a `tempfile::TempDir`.
  2. Writes inputs and `params.yaml`.
  3. Invokes the handler against that directory.
  4. Asserts on the written `outputs/<name>/...` files.

### 0.6 Expose a test-friendly entrypoint per block

`spade::run` reads from the current working directory, which is awkward in unit tests. Each block module exposes two functions:

- [DONE] `pub fn entry()` — the production entrypoint, calls `spade::run(handler)`.
- [DONE] `pub fn run_in(base: &Path) -> Result<()>` — test-friendly variant that uses `spade::scanning::build_args_from(base)` and `spade::output::write_outputs_to(result, base, manifest)` directly.

Both call the same internal `fn handler(args: Args) -> Result<T>`. The pattern is:

```rust
// src/filter_rows.rs
use spade::{Args, Result, TabularFile};

fn handler(args: Args) -> Result<TabularFile> { /* ... */ }

pub fn entry() { spade::run(handler); }

pub fn run_in(base: &std::path::Path) -> Result<()> {
    let args = spade::scanning::build_args_from(base)?;
    let out = handler(args)?;
    spade::output::write_outputs_to(out, base, None)?;
    Ok(())
}
```

- [DONE] Apply this pattern to every block module.

---

## Phase 1 — Utility blocks (simplest, prove the toolchain)

Start with the two format-conversion blocks. They exercise the shared table helpers end-to-end but have trivial logic, so they're the right way to validate the scaffolding.

### 1.1 `base.csv_to_parquet`

- [DONE] Fill in `blocks/csv_to_parquet.yaml` with `id`, `version: 0.1.0`, `kind: standard`, `network: false`, `description`, and the inputs/outputs from `BLOCKS.md` §4.
- [DONE] Create `src/csv_to_parquet.rs` with:
  - [DONE] Handler signature: `fn handler(args: Args) -> Result<TabularFile>`.
  - [DONE] Read `args.input::<TabularFile>("table")`.
  - [DONE] Read `delimiter: String` (default `","`) and `has_header: bool` (default `true`) from params, using `args.has_param` to gate the defaults.
  - [DONE] Parse the CSV via `CsvReadOptions::default().with_has_header(has_header).with_separator(delimiter_u8).try_into_reader_with_file_path(...)`.
  - [DONE] Write to `"result.parquet"` in the cwd using `common::table::write_parquet`.
  - [DONE] Return `TabularFile::new("result.parquet")`.
- [DONE] Register the module in `lib.rs` and add a match arm in `main.rs`.
- [DONE] Integration test: write a 3-row CSV, run `csv_to_parquet::run_in(tmp)`, read the Parquet back with Polars, assert row count and column names.
- [DONE] Error-path test: supply a non-CSV file, assert `BaseError::Polars`.

### 1.2 `base.parquet_to_csv`

- [DONE] Mirror 1.1 with input format Parquet, output CSV. Use `CsvWriter` with the configured delimiter and `include_header`.
- [DONE] Handler returns `TabularFile::new("result.csv")`. Because the default output name for `TabularFile` is `"table"` but the manifest names this output `"result"`, rely on the runtime's manifest-driven renaming by supplying the manifest to `write_outputs_to` in `run_in` (load it via `spade::output::read_block_manifest_from(base)`).
- [DONE] Integration test: round-trip a small DataFrame and confirm cells are preserved.

### 1.3 Sanity: compile and run the binary

- [DONE] `cargo build --release` produces `target/release/base`.
- [DONE] Set up a throwaway working directory and run `target/release/base csv_to_parquet` to confirm the dispatcher wires up correctly.
- [DONE] Run `cargo test` — all Phase 1 tests should pass.

---

## Phase 2 — Tabular core blocks

These are the workhorses. They all follow the same outline: read one table, transform it with Polars, write one Parquet file.

### 2.1 `base.filter_rows`

- [DONE] Manifest at `blocks/filter_rows.yaml` per `BLOCKS.md` §1.
- [DONE] `src/filter_rows.rs` handler:
  - [DONE] Load the table as a `LazyFrame` (lazy because filters are cheap to compose).
  - [DONE] Read `expression: String` from params. Non-empty required; return `BaseError::BadExpression` if missing or empty.
  - [DONE] Use `polars_sql::SQLContext` to parse the expression. Register the loaded frame as table `"t"`, then execute `SELECT * FROM t WHERE <expression>`. (This is more robust than a custom expression parser and gives users standard SQL.)
  - [DONE] Collect and write to `"result.parquet"`.
- [DONE] Integration tests:
  - [DONE] Numeric predicate: `age > 30` on a 5-row frame, assert the right subset.
  - [DONE] String predicate with quotes: `state = 'NY'`.
  - [DONE] Boolean combination: `age > 30 AND state = 'NY'`.
  - [DONE] Invalid SQL: `age >>> 30` — assert `BaseError::BadExpression`.
  - [DONE] Column that doesn't exist — assert `BaseError::Polars`.

### 2.2 `base.select_columns`

- [DONE] Manifest at `blocks/select_columns.yaml`.
- [DONE] Handler reads `columns: String` (CSV list) and `mode: String` (`"keep"` or `"drop"`, default `"keep"`).
- [DONE] `"keep"` path: `lf.select(column_names.iter().map(col).collect::<Vec<_>>())`.
- [DONE] `"drop"` path: compute the complement against `lf.schema()` and select that.
- [DONE] Validate that every requested column exists in the schema; return `BaseError::SchemaMismatch { missing: [...] }` if not.
- [DONE] Integration tests: keep subset, drop subset, missing column error, unknown mode error.

### 2.3 `base.aggregate`

This is the heaviest block; allocate time for it.

- [DONE] Define an `AggregationSpec` struct in `src/aggregate.rs`:
  ```rust
  #[derive(Deserialize)]
  pub struct AggregationSpec {
      column: String,
      function: String,
      #[serde(default)]
      p: Option<f64>,  // for percentile
      #[serde(rename = "as", default)]
      alias: Option<String>,
  }
  ```
- [DONE] Write `fn build_expr(spec: &AggregationSpec) -> Result<Expr>` that maps `function` to a Polars expression:
  - [DONE] `"mean"` → `col(&spec.column).mean()`.
  - [DONE] `"median"` → `col(&spec.column).median()`.
  - [DONE] `"mode"` → `col(&spec.column).mode().first()` (mode returns a list; we take the first).
  - [DONE] `"sum"` → `.sum()`.
  - [DONE] `"min"` / `"max"` → `.min()` / `.max()`.
  - [DONE] `"count"` → `.count()`.
  - [DONE] `"count_distinct"` → `.n_unique()`.
  - [DONE] `"std"` → `.std(1)`. `"var"` → `.var(1)`.
  - [DONE] `"percentile"` → requires `p`; `.quantile(p.into(), QuantileMethod::Linear)`. Error if `p` is missing or outside `[0, 1]`.
  - [DONE] Unknown function → `BaseError::InvalidAggregation`.
  - [DONE] Wrap the final expression in `.alias(&alias.unwrap_or(default_name_for(spec)))`.
- [DONE] Handler flow:
  - [DONE] Read table.
  - [DONE] Parse `aggregations: String` via `common::params::parse_json_list::<AggregationSpec>`.
  - [DONE] Parse `group_by: String` via `parse_csv_list`.
  - [DONE] If `group_by` is empty: `lf.select(exprs)`. Otherwise: `lf.group_by(group_cols).agg(exprs)`.
  - [DONE] Collect, write to `"result.parquet"`.
- [DONE] Integration tests, one per function supported, plus:
  - [DONE] No grouping, single aggregation → 1-row result.
  - [DONE] Grouping by two columns with mean and count → multi-row result, asserts stable ordering (sort by group keys in handler for determinism).
  - [DONE] Percentile without `p` → `InvalidAggregation` error.
  - [DONE] Percentile with `p = 1.5` → `InvalidAggregation` error.
  - [DONE] Unknown function → `InvalidAggregation` error.
  - [DONE] Malformed JSON in `aggregations` → `BaseError::Json`.

### 2.4 `base.group_by`

- [DONE] Manifest at `blocks/group_by.yaml`.
- [DONE] Handler is a thin shim: parse `group_columns` (required, non-empty) and `aggregations`, then delegate to the same core routine as `aggregate::run_grouped(...)`.
- [DONE] Refactor `aggregate.rs` to expose `pub fn run_grouped(df: LazyFrame, group_cols: &[String], specs: &[AggregationSpec]) -> Result<DataFrame>` and call it from both blocks.
- [DONE] Integration test: identical pipeline and data to one of the 2.3 grouped tests, assert identical output.
- [DONE] Decision checkpoint: if the team agrees `base.aggregate` alone is enough, delete the `group_by` manifest, module, and dispatcher arm. Leave a one-line mention in `BLOCKS.md`.

---

## Phase 3 — Map blocks

Map blocks write an `expansion.yaml` manifest and, for non-file inputs, also materialise per-item files.

### 3.1 `base.map_files` (already stubbed)

- [DONE] Replace the empty `blocks/map_files.yaml` with the full manifest from `BLOCKS.md` §2.
- [DONE] Replace the stub in `src/map_files.rs`:
  - [DONE] Handler returns `()` (the runtime library doesn't model `expansion` as a first-class type yet; write the file directly under `outputs/manifest/expansion.yaml`).
  - [DONE] Read `source: FileCollection` via `args.input`.
  - [DONE] Sort the paths lexicographically (determinism — already done by `scan_inputs`, but re-sort defensively in case input ordering assumptions change).
  - [DONE] For each path, compute `key` = file stem (no extension). Emit `{ path: <relative path>, key: <stem> }`.
  - [DONE] Serialize to YAML with `serde_yaml::to_string(&ExpansionManifest { items })`. Write to `outputs/manifest/expansion.yaml`. Create the directory first.
  - [DONE] Document in a module comment that this block intentionally bypasses `IntoOutput` because the runtime doesn't model expansion yet.
- [DONE] Define `#[derive(Serialize)] struct ExpansionManifest { items: Vec<ExpansionItem> }` and `struct ExpansionItem { path: String, key: String }` in `common::expansion` so `map_list` and `map_range` can reuse them.
- [DONE] Integration test: create 3 input files with gaps in naming (`a.tif`, `c.tif`, `b.tif`), run the block, parse the written YAML, assert the items are sorted by filename.

### 3.2 `base.map_list`

- [DONE] Manifest at `blocks/map_list.yaml`.
- [DONE] Handler flow:
  - [DONE] Read `values: String` param.
  - [DONE] Parse as `Vec<serde_json::Value>`; accept strings and numbers, reject objects/arrays (error via `BaseError::BadExpression`).
  - [DONE] For each value, write a tiny JSON file to `outputs/manifest/items/<index>.json` with contents `{"value": <value>}`. Use zero-padded indices (`00`, `01`, …) so filesystem ordering matches iteration.
  - [DONE] Emit one expansion item per value: `path = "outputs/manifest/items/<idx>.json"`, `key = <value as string, sanitised>`.
  - [DONE] Write `outputs/manifest/expansion.yaml`.
- [DONE] Integration tests:
  - [DONE] `["NY", "MI", "CA"]` → 3 items written, expansion.yaml lists them in order.
  - [DONE] `[1, 2, 3]` → numeric keys serialized as strings (`"1"`, `"2"`, `"3"`).
  - [DONE] `[]` empty list → empty expansion, no items directory. Decide up front whether this is an error or a no-op; I recommend erroring with `BaseError::EmptyCollection` so downstream blocks don't silently produce zero invocations.
  - [DONE] Malformed JSON → `BaseError::Json`.

### 3.3 `base.map_range`

- [DONE] Manifest at `blocks/map_range.yaml`.
- [DONE] Handler flow:
  - [DONE] Read `start: f64`, `end: f64`, and optional `step: f64` (default `1.0`).
  - [DONE] Validate: `step != 0`, and the range is finite (no `NaN`/`Inf`). Error if start == end.
  - [DONE] Generate the sequence. Because `f64` iteration is error-prone, detect when all three values are integer-valued (`x.fract() == 0.0 && x.is_finite()`) and iterate as `i64` to avoid accumulated drift. Otherwise iterate in `f64` with a safety cap (e.g. refuse to emit more than 1,000,000 items — return `BaseError::BadExpression`).
  - [DONE] Reuse the item-materialisation logic from `map_list` (extract to `common::expansion::materialise_scalar_items(values) -> Result<Vec<ExpansionItem>>`).
- [DONE] Integration tests:
  - [DONE] `start=0, end=5, step=1` → 5 items.
  - [DONE] `start=0, end=1, step=0.25` → 4 items.
  - [DONE] `step=0` → error.
  - [DONE] `start == end` → error.
  - [DONE] Range that would emit >1M items → error.

### 3.4 Decision checkpoint

- [DONE] If the team decides `map_list` plus external JSON generation is enough, delete `map_range` and note it in `BLOCKS.md`.

---

## Phase 4 — Reduce blocks

Reduce blocks consume a `collection` of files (the outputs of N mapped invocations) and produce one or more outputs.

### 4.1 `base.reduce_collection`

- [DONE] Manifest at `blocks/reduce_collection.yaml`.
- [DONE] Handler signature: `fn handler(args: Args) -> Result<FileCollection>`.
- [DONE] Body: just pass through — `let items: FileCollection = args.input("items")?; Ok(items)`. The runtime will copy each file into `outputs/result/`.
- [DONE] Integration test: create 3 files under `inputs/items/`, run the block, assert 3 matching files appear under `outputs/result/` with the same contents.
- [DONE] This block is the canonical "minimum viable reducer" — add a module-level doc comment saying so, since it reads like a no-op but is load-bearing for pipelines.

### 4.2 `base.reduce_stack`

- [DONE] Manifest at `blocks/reduce_stack.yaml`.
- [DONE] Handler reads `tables: TabularFileCollection` and `strict: bool` (default `false`).
- [DONE] Strict path: load all frames, verify schemas match exactly, then `polars::functions::concat_df` (eager) or `concat` on lazy frames. Error `BaseError::SchemaMismatch` on any mismatch.
- [DONE] Non-strict path: use `concat_lf_diagonal(..., UnionArgs::default())` which unions schemas and fills missing columns with null.
- [DONE] Deterministic ordering: sort `paths` before loading so the stacked order is stable across reruns. (`scan_inputs` already sorts, but re-sort defensively.)
- [DONE] Integration tests:
  - [DONE] 3 tables with identical schemas → stacked correctly, `strict=true` passes.
  - [DONE] 3 tables with one extra column in the middle file, `strict=false` → nulls fill in the other two.
  - [DONE] Same as above with `strict=true` → `SchemaMismatch` error.
  - [DONE] Empty collection (zero files) → `EmptyCollection` error.

### 4.3 `base.reduce_join`

- [DONE] Manifest at `blocks/reduce_join.yaml`.
- [DONE] Handler reads `tables: TabularFileCollection`, `on: String` (CSV list, required non-empty), `how: String` (default `"inner"`).
- [DONE] Parse `how` into `JoinType` — `inner`, `left`, `right`, `outer` (map to `JoinType::Full`). Error on unknown values.
- [DONE] Load all frames (lazy), then fold left-to-right: `acc.join(next, join_exprs.clone(), join_exprs.clone(), how.clone().into())`.
- [DONE] Key validation: confirm every key column exists in every frame before joining. Collect missing keys per frame and produce one clear `SchemaMismatch` error listing all of them at once.
- [DONE] Integration tests:
  - [DONE] 2 frames, inner join on `id` → standard result.
  - [DONE] 3 frames → chain of joins.
  - [DONE] Left join with non-overlapping keys → null-filled columns.
  - [DONE] Missing key column in one frame → one error listing the offending frame(s).
  - [DONE] Empty collection → `EmptyCollection`.
  - [DONE] Single-table collection (edge case) → return the single table unchanged.

---

## Phase 5 — Integration and polish

With all blocks implemented, tie the collection together.

### 5.1 Cross-block integration tests

Under `tests/pipelines/`, write end-to-end tests that chain block handlers to simulate a real pipeline, skipping the scheduler. Each test writes intermediate files under a single tempdir tree.

- [DONE] **Tabular chain**: `csv_to_parquet` → `filter_rows` → `select_columns` → `aggregate`. Assert the final aggregated table's row count and cell values.
- [DONE] **Map/reduce chain**: `map_files` over a 4-file input, then treat each item as standalone input to `select_columns`, then `reduce_stack` the outputs. Assert final row count = sum of per-file row counts.
- [DONE] **Mixed**: `map_list(["NY","MI"])` feeding into a block that reads the per-item JSON, then `reduce_collection`. Assert both JSON files reappear in the output collection.

### 5.2 Manifest validation

- [DONE] Run `spade check` (the CLI from `../../cli/`) against the collection. Fix anything it complains about — missing fields, bad `id`s, invalid input/output types, map blocks without `expansion`, reduce blocks without `collection`.
- [DONE] Confirm every manifest has `description` on the block and on every input and every output. The CLI doesn't enforce this but the web UI displays them.

### 5.3 Documentation

- [DONE] Add a top-level `README.md` listing the blocks with a one-line description each and a pointer to `BLOCKS.md` for full specs. Keep it short — the manifests are the source of truth.
- [DONE] Add module-level `//!` doc comments to every `src/*.rs` block file explaining the block's role and any non-obvious implementation choices (e.g. why `filter_rows` routes through `polars-sql` rather than parsing by hand).

### 5.4 Performance sanity check

- [DONE] Benchmark `aggregate` on a 10M-row Parquet file with one numeric and two group columns. Should complete in single-digit seconds on a laptop; if not, profile and consider enabling Polars' streaming engine (`lf.with_streaming(true)`) or the new streaming API in recent Polars versions.
- [DONE] Benchmark `reduce_stack` on 100 files of 100k rows each. Same target.
- [DONE] No commitment to absolute numbers — these are smell tests, not SLOs.

### 5.5 CI wiring

- [DONE] Add a CI job (wherever this monorepo's CI lives — out of scope to set up here if not already present) that runs `cargo fmt --check`, `cargo clippy -- -D warnings`, `cargo test`, and `spade check` on this collection.

---

## Phase 6 — Final release checks

- [DONE] Bump `version` in `Cargo.toml` to `0.1.0` and the same version in every `blocks/*.yaml`.
- [DONE] `cargo build --release` produces a `base` binary with reasonable size (< 50 MB stripped).
- [DONE] Install into `~/.spade/blocks/base/0.1.0/` following the CLI's install convention (usually handled by `spade install .`) and run one pipeline end-to-end against an installed copy, not a local `cargo run`.
- [DONE] Tag the commit and open a PR.

---

## Work-order summary

The dependency order between phases is strict:

1. **Phase 0** unblocks everything.
2. **Phase 1** validates the toolchain before investing in the hard blocks.
3. **Phases 2, 3, 4** are independent of each other and can be worked in parallel if multiple contributors are available.
4. **Phase 5** requires all of the above.
5. **Phase 6** is the final gate.

Rough time estimate, one contributor, end-to-end: **2–3 focused days** for Phases 0–4, another **1 day** for Phase 5 polish and integration tests. `aggregate` and `reduce_join` are the two blocks most likely to blow through estimates because the expression and join APIs have a learning curve if you haven't used Polars recently.

---

## Open design decisions to resolve before coding

Flag any of these back to the maintainer if you hit them during implementation:

- [DONE] **`group_by` vs. `aggregate` collapse.** Keep both for pipeline readability, or drop `group_by`? Decided before coding Phase 2.
- [DONE] **`map_range` vs. `map_list` collapse.** Same question for Phase 3.
- [DONE] **List params as JSON vs. native YAML lists.** The spec says scalar params only are supported, but `params.yaml` is YAML, and the runtime's `Args::param` goes through `serde_yaml::from_value` which happily deserialises into `Vec<T>`. If it works (confirm with a quick unit test against the runtime), prefer native lists for ergonomics and drop the JSON-string workaround. If not, stick with JSON strings.
- [DONE] **SQL parser for `filter_rows`.** Going through `polars-sql` means users write SQL-like predicates. Alternative: accept a Polars expression DSL as a string and evaluate via `polars_plan::dsl`. SQL is more familiar; the DSL is more powerful. Default to SQL unless the team has a strong opinion.
- [DONE] **Expansion-as-type support.** The runtime library does not currently model `type: expansion` as a first-class type. Map blocks work around this by writing `expansion.yaml` directly. If someone adds an `ExpansionManifest` type to `libs/rust`, revisit map blocks to use it.
