+++
title = "Execution Model"
description = "How Spade schedules, executes, and caches block invocations."
weight = 5
+++

This page describes how Spade takes a pipeline definition and turns it into running computations. Understanding the execution model helps you reason about performance, caching behavior, and how to diagnose problems when things go wrong.

## Overview

When you run a pipeline, Spade goes through these stages:

1. **Validation** — Check the pipeline for errors (missing blocks, invalid references, cycles)
2. **Scheduling** — Build a dependency graph and determine execution order
3. **Execution** — Run blocks as sandboxed subprocesses, routing data between them
4. **Caching** — Store outputs for reuse in future runs

## Scheduling

The **scheduler** reads the pipeline's dependency graph and determines which blocks are ready to run. A block is ready when all of its upstream dependencies have completed successfully.

The scheduler maintains a work queue. At any point in time, every block in the pipeline is in one of these states:

- **Waiting** — One or more upstream dependencies have not yet completed
- **Ready** — All dependencies are satisfied; the block can be dispatched to a worker
- **Running** — A worker is currently executing the block
- **Completed** — The block finished successfully and its outputs are available
- **Failed** — The block exited with a non-zero exit code

The scheduler continuously moves blocks from "Waiting" to "Ready" as their dependencies complete, and dispatches ready blocks to available workers. Blocks that have no dependencies on each other run in parallel, limited only by the number of available workers.

## Workers and working directory setup

A **worker** receives a block assignment from the scheduler and prepares the execution environment. The setup process is:

1. **Create the working directory** with subdirectories: `inputs/`, `outputs/`, `logs/`
2. **Write `params.yaml`** containing the block's parameters from the pipeline's `args`
3. **Write `invocation.yaml`** containing metadata (block ID, run ID, map context if applicable)
4. **Symlink inputs** from upstream block outputs into `inputs/`. For each declared input, the worker creates a symlink pointing to the corresponding output file or directory from the upstream block. This avoids copying large files.

The result is a self-contained working directory:

```
<working-directory>/
  params.yaml
  invocation.yaml
  inputs/
    image -> /path/to/upstream/outputs/satellite_image.tif
    model -> /path/to/upstream/outputs/trained_model.pkl
  outputs/
  logs/
```

## Sandbox execution

Once the working directory is ready, the worker launches the block's entrypoint as a **subprocess inside a sandbox**. Spade uses `isolate` to restrict what the subprocess can do:

- **Filesystem** — The block can only access its own working directory. It cannot read or write files elsewhere on the system.
- **Memory** — The block is limited to a configurable amount of memory. If it exceeds this limit, the process is terminated.
- **CPU** — The block runs with a configurable CPU time limit to prevent runaway processes.
- **Network** — Network access is disabled by default. Blocks that declare `network: true` in their manifest are given network access.

The subprocess runs the block's entrypoint (e.g., a Python script, a compiled Go binary). The block reads its inputs, performs its computation, and writes its outputs. Standard output and standard error are captured to `logs/stdout.log` and `logs/stderr.log`.

## Completion and reporting

When the subprocess finishes:

- **Exit code 0** (success) — The worker verifies that all declared outputs exist in `outputs/`, reports success to the scheduler, and the outputs become available for downstream blocks.
- **Non-zero exit code** (failure) — The worker reports the failure to the scheduler. See [Error handling](#error-handling) below.

## Caching

Spade caches block outputs to avoid redundant computation. Before running a block, Spade computes a **cache key** from:

- The block's `id` (e.g., `raster.reproject`)
- The block's `version` (e.g., `0.2.1`)
- A content hash of every input file (computed from file contents, not file paths)
- The contents of `params.yaml`
- A hash of the runtime environment (language version, dependency versions)

If a cache entry with a matching key already exists, Spade **skips execution entirely** and uses the cached outputs. The cached outputs are symlinked into the working directory as if the block had just run, so downstream blocks see no difference.

This means:

- **Identical inputs produce cached results** — Re-running a pipeline with the same data and parameters is nearly instantaneous after the first run.
- **Any change invalidates the cache** — Changing a parameter, updating an input file, or bumping the block version causes re-execution.
- **Cache keys are content-based** — Renaming an input file but keeping the same content still produces a cache hit, because the hash is computed from file contents.

Cached outputs are stored in `~/.spade/cache/`.

## Local execution

The simplest way to run a pipeline is locally with `spade run`:

```bash
spade run my-pipeline.yaml
```

In local mode, Spade runs a single-instance scheduler and a pool of workers on your machine. The number of parallel workers defaults to the number of available CPU cores but can be configured:

```bash
spade run --workers 4 my-pipeline.yaml
```

All block working directories, logs, and cached outputs are stored locally under `~/.spade/`.

## Distributed execution

For large workloads, Spade supports distributed execution across multiple machines. In this mode:

- A **scheduler** runs on one machine and coordinates the pipeline
- Multiple **workers** run on separate machines, each pulling block assignments from the scheduler
- All machines must share a **common filesystem** (e.g., NFS, a network-attached storage volume) so that input symlinks and cached outputs are accessible everywhere

The scheduler and workers communicate to dispatch work and report results. From the pipeline's perspective, distributed execution behaves identically to local execution: the same dependency resolution, sandboxing, and caching rules apply. The only difference is that blocks can run on different machines, increasing total throughput.

## Error handling

When a block fails (exits with a non-zero exit code), the following happens:

1. **The pipeline halts.** The scheduler stops dispatching new blocks. Blocks that are already running are allowed to finish, but no new blocks are started.
2. **Logs are preserved.** The failed block's `logs/stdout.log` and `logs/stderr.log` are kept in the working directory for inspection.
3. **Spade reports the failure.** The CLI displays which block failed, its exit code, and the path to its logs.

To debug a failure:

```bash
# Run with preserved working directories
spade run --keep-work-dir my-pipeline.yaml

# Check the failed block's logs
cat ~/.spade/pipelines/<run-id>/<block-id>/logs/stderr.log
```

Blocks that completed successfully before the failure are cached normally. When you fix the failing block and re-run the pipeline, the successful blocks are skipped (served from cache) and only the previously-failed block and its downstream dependents re-execute.

## Summary of the execution lifecycle

| Stage | What happens |
|-------|-------------|
| Validation | Pipeline YAML is parsed, references are checked, cycles are detected, types are verified |
| Scheduling | Dependency graph is built, ready blocks are identified |
| Cache check | For each ready block, compute cache key and check for a hit |
| Working directory setup | Create directories, write params.yaml, symlink inputs |
| Sandbox execution | Launch subprocess inside `isolate` with filesystem/memory/CPU/network restrictions |
| Output capture | stdout/stderr written to logs/, outputs verified |
| Completion | Cache outputs, notify scheduler, unblock downstream blocks |
| Error (if any) | Halt pipeline, preserve logs, report failure |
