# Stats Blocks

Spade block collection of common statistical operations, written in R. Every
block reads a tabular (CSV) input and emits a JSON result, so the blocks compose
behind any source that produces a CSV (e.g. `data.read`) and feed any consumer
that accepts JSON.

The collection name is `stats`, so blocks are referenced as `stats.<name>` in
pipelines (e.g. `stats.summary`, `stats.t_test`).

## Blocks

| Block               | Description                                                                 |
| ------------------- | --------------------------------------------------------------------------- |
| `stats.summary`     | Per-column descriptive statistics (n, mean, sd, min, quartiles, max, missing). |
| `stats.correlation` | Correlation matrix over numeric columns (`pearson`/`spearman`/`kendall`).   |
| `stats.frequency`   | One-way frequency table, or a contingency table when `by` is set.           |
| `stats.t_test`      | One-sample (vs `mu`) or two-sample/paired Student's t-test.                  |
| `stats.anova`       | One-way analysis of variance of a numeric response across a grouping column. |
| `stats.chisq_test`  | Pearson's chi-squared test of independence between two categorical columns.  |

All blocks take a `data` input (`type: file`, `format: CSV`) and write a single
`result` output (`type: json`). Remaining configuration is passed as scalar
`args` (column names, method, alternative, etc.) — see each `blocks/*.yaml`
manifest for the full input list and descriptions.

## Runtime library

Handlers use the `spade` R authoring library (`libs/R` in this monorepo):
typed inputs are declared with `spade_types()`, and `run(handler)` loads
`params.yaml`, scans `inputs/`, and writes the returned `JsonFile` to
`outputs/result/`. Because the library only *copies* a `JsonFile`, each handler
serializes its result to disk itself with `jsonlite::write_json(..., na = "null")`.

## Setup and install

Dependencies are declared in `DESCRIPTION` (`Imports: jsonlite`) and pinned in the
committed `pkg.lock`. `spade install` uses `pak` to install them — plus the local
`spade` R package — into the per-user R library (`R_LIBS_USER`), the library the
worker's isolate sandbox binds onto R's search path. Without this step
`library(spade)` cannot resolve inside the sandbox.

```bash
cd blocks/stats
spade check          # validate all manifests
spade install .      # pak-installs deps + spade, then registers the blocks
```

The local `spade` source defaults to `../../libs/R`; override with the
`SPADE_R_LIB_SRC` environment variable when installing from a checkout whose
layout differs. (`spade` is installed locally until it is published.)

## Environment notes

- R 4.x. Dependencies are resolved by `pak` from `DESCRIPTION`/`pkg.lock`.
- `jsonlite` is a direct dependency; `yaml` comes in transitively with `spade`.
- Handlers rely only on base R plus the `stats` package for the statistics
  themselves, so the dependency surface is intentionally small.

## Example pipeline

```yaml
name: summary_demo
version: "1.0"
blocks:
  - id: "@read"
    name: data.read
    inputs: []
    args:
      uri: /abs/path/to/data.csv
      format: CSV
  - id: "@summary"
    name: stats.summary
    inputs:
      - "@read"
    args:
      columns: "x,y"   # omit to summarize all numeric columns
```

Integration tests for every block live in `test/` (see `test/generate.py`,
`stats_*` pipelines).
