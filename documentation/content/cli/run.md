+++
title = "spade run"
description = "Execute a pipeline locally."
weight = 6
+++

The `spade run` command executes a pipeline on the local machine using the single-instance scheduler. It loads the pipeline, validates it, resolves all block dependencies, and executes blocks in topological order.

## Usage

```bash
spade run <pipeline.yaml>
spade run --no-ui <pipeline.yaml>
spade run --keep-work-dir <pipeline.yaml>
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--no-ui` | `false` | Disable the BubbleTea terminal interface; use simple line-by-line output |
| `--keep-work-dir` | `false` | Preserve the pipeline working directory after execution completes |

## Execution flow

### 1. Load and validate

The pipeline YAML file is parsed and each block's `name` is looked up in the local registry (`~/.spade/registry.db`). The full set of pipeline validation checks (the same ones used by [`spade check`](/cli/check/)) is run before any block executes. If validation fails, the command prints the errors and exits with status 1.

### 2. Set up the working directory

A working directory is created at:

```
~/.spade/pipelines/<pipeline-uuid>/
```

Inside this directory, each block invocation gets its own subdirectory named by its invocation UUID, containing:

```
<invocation-uuid>/
  inputs/     # Symlinks to upstream block outputs
  outputs/    # Files written by the block
  logs/       # Execution logs
```

The block receives a `params.yaml` file in its working directory containing the `args` values from the pipeline definition.

Input files from upstream blocks are made available via symlinks in the `inputs/` directory, organized by input name.

### 3. Schedule and execute

Blocks are executed in dependency order determined by topological sort of the pipeline's DAG. The scheduler produces block invocations one at a time:

- **Source blocks** (those with no inputs) are executed first.
- Each subsequent block runs only after all of its dependencies have completed.
- **Map blocks** produce an expansion manifest, which causes the scheduler to create multiple downstream invocations (one per expansion item).
- **Reduce blocks** run after all mapped invocations in their context have completed.

For each block, the scheduler resolves the executable via the registry entry:

| Language | Executable | Arguments |
|----------|-----------|-----------|
| Rust | `<install-path>/<collection-name>` | `<entrypoint>` |
| Go | `<install-path>/<collection-name>` | `<entrypoint>` |
| TypeScript | `<install-path>/<collection-name>` | `<entrypoint>` |
| Python | `uv` | `run <entrypoint>` |
| R | `Rscript` | `<entrypoint>` |

### 4. Cache check

Before executing each block, the runner computes a cache key from:

- The block ID and version
- SHA-256 hashes of all input files
- The block's arguments

If a cache entry exists in `~/.spade/cache/`, the cached outputs are restored to the working directory and the block is marked as complete without re-executing. The output shows `(cached)` for these blocks.

### 5. Collect outputs

After a block completes successfully, its `outputs/` directory is scanned and each output is hashed (SHA-256). These hashes are stored for downstream cache key computation and used to verify input integrity for dependent blocks. On success, the outputs are also stored in the cache for future runs.

## Output

With the default BubbleTea UI, progress is shown with animated indicators:

```
Running pipeline 'reproject-example'...
  [1/2] data.sentinel2 .......... done (3.2s)
  [2/2] raster.reproject ........ done (1.1s)
Pipeline complete! (4.3s total)
```

With `--no-ui`, output is simpler line-by-line text:

```
Loaded pipeline: reproject-example (019cf4bc-0000-7000-0000-000000000000)
Executing pipeline with 2 block(s)...
  [1/2] data.sentinel2 running...
  [1/2] data.sentinel2 complete
  [2/2] raster.reproject running...
  [2/2] raster.reproject complete

Pipeline complete: 2 block(s) executed in 4.312s
```

When blocks are served from cache, the output reflects this:

```
  [1/2] data.sentinel2 (cached)
```

## Failure behavior

If a block exits with a non-zero status, the pipeline halts immediately. The error message from the failed block is printed to stderr, and the command exits with status 1:

```
Block data.sentinel2 failed: process exited with status 1
```

Blocks that completed successfully before the failure retain their cached outputs. On the next run, those blocks will be served from cache, and only the failed block (and its downstream dependents) will re-execute.

## Working directory cleanup

By default, the working directory (`~/.spade/pipelines/<pipeline-uuid>/`) is removed after the pipeline finishes, whether it succeeds or fails. Use `--keep-work-dir` to preserve it for debugging:

```bash
spade run --keep-work-dir my-pipeline.yaml
```

Inside the preserved working directory, you can inspect each block's `inputs/`, `outputs/`, and `logs/` subdirectories to understand what data flowed through the pipeline.

## See also

- [`spade check`](/cli/check/) for validating a pipeline without executing it
- [`spade install`](/cli/install/) for installing the blocks referenced by a pipeline
- [Your First Pipeline](/getting-started/first-pipeline/) for a step-by-step tutorial
