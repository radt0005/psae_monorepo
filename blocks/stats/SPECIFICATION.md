# Statistics Block Specifications

This document specifies the `stats` block collection for the spade system. For
more information on the core system, see `../../spec/`; for block- and
pipeline-authoring conventions, see the skill at `../../skills/spade`.

This collection provides common **statistical operations** — descriptive
statistics and hypothesis tests — over tabular data. It is written in **R**,
which has first-class, well-tested implementations of these methods in its base
`stats` package, so the blocks are thin, dependency-light wrappers around
functions statisticians already trust (`summary`, `cor`, `t.test`, `aov`,
`chisq.test`, `table`).

## Design conventions

Every block in this collection follows the same shape, so they compose
uniformly behind any CSV source and ahead of any JSON consumer:

- **Input:** a single tabular file, `data` (`type: file`, `format: CSV`). Read
  with base R's `read.csv`.
- **Output:** a single `result` (`type: json`). Because the spade R library
  only *copies* a returned `JsonFile`, each handler serializes its own result
  with `jsonlite::write_json(..., auto_unbox = TRUE, na = "null")` and returns
  `JsonFile(path)`. `na = "null"` ensures missing values become JSON `null`
  rather than the string `"NA"`.
- **Configuration:** all other parameters are scalars passed via pipeline
  `args` (column names, methods, test options), never wired inputs.
- **Typed handlers:** R carries no type information in a function signature, so
  each handler declares `spade_types()` (and a `spade_description`) so the
  runtime constructs the right `TabularFile` for `data` and the library can
  generate a manifest.

Blocks validate their parameters and fail with a clear, non-zero error (which
halts the pipeline) when a referenced column is missing, is the wrong type, or a
test precondition is violated (e.g. a two-sample t-test needing exactly two
groups).

## Blocks

### Descriptive statistics

1. **`summary`** — per-column descriptive statistics for the numeric columns:
   `n`, `missing`, `mean`, `sd`, `min`, `q25`, `median`, `q75`, `max`. An
   optional `columns` arg (comma-separated) restricts the set; otherwise all
   numeric columns are used. Result is keyed by column name.
2. **`correlation`** — a correlation matrix over numeric columns. `method` is
   one of `pearson` (default), `spearman`, or `kendall`; `columns` optionally
   restricts the set (at least two are required). Uses pairwise-complete
   observations. Result is a nested object keyed by column, plus the method and
   column order.
3. **`frequency`** — a frequency table for a categorical `column`. With no `by`,
   a one-way table of counts and proportions per level; with `by`, a contingency
   table nested as `{ column_level: { by_level: count } }`.

### Hypothesis tests

4. **`t_test`** — Student's t-test on a numeric `value_column`. With a
   `group_column` (which must have exactly two levels) it runs a two-sample test
   (Welch by default, or paired when `paired: true`); otherwise it runs a
   one-sample test against `mu`. Honors `alternative`
   (`two.sided`/`less`/`greater`) and `conf_level`. Result reports the method,
   statistic, degrees of freedom, p-value, confidence interval, and estimate(s).
5. **`anova`** — one-way analysis of variance of a numeric `value_column` across
   the levels of `group_column`. Result is the ANOVA table (df, sum of squares,
   mean squares, F, p) for the group term and residuals.
6. **`chisq_test`** — Pearson's chi-squared test of independence between two
   categorical columns, `column` and `by`. `correct` toggles Yates' continuity
   correction (default `true`, only affects 2x2 tables). Result reports the
   statistic, degrees of freedom, p-value, and the observed contingency table.

## Runtime and packaging

Handlers use the `spade` R authoring library (`../../libs/R`): typed inputs via
`spade_types()`, and `run(handler)` to load `params.yaml`, scan `inputs/`, and
write the returned `JsonFile` into `outputs/result/`.

`setup.R` is the collection's build step, run by `spade install`. It installs
the runtime dependencies (`jsonlite`, `yaml`) and the local `spade` package into
the per-user R library (`R_LIBS_USER`) — the library the worker's isolate
sandbox binds onto R's search path. Without this step `library(spade)` cannot
resolve at block-execution time. The `spade` source defaults to `../../libs/R`,
overridable via the `SPADE_R_LIB_SRC` environment variable.

Dependencies are intentionally minimal: base R plus the `stats` package for the
statistics themselves, `jsonlite` for output, and `yaml` (a `spade` dependency).

## Testing

Integration pipelines for every block live in `../../test/` — see
`test/generate.py` (`_stats_pipelines`, the `stats_data.csv` fixture, and the
`stats.*` entries in `BLOCK_DEFAULTS`) and the generated `stats_*` pipelines.
Each exercises one block end-to-end behind `data.read` through the isolate
sandbox.

## Hints

See `../../skills/spade` for the skill describing how to author blocks and
pipelines for the spade system.
