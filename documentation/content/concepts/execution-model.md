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

## Storage model: no shared filesystem

Workers do not share a filesystem with each other or with the scheduler. This is a deliberate architectural choice, not a current limitation: Spade considered a shared network filesystem (NFS or similar) and rejected it because the overhead and operational complexity of a shared volume outweighed the benefit at the scale Spade targets. Instead, data flowing between blocks moves through **object storage**, and each worker keeps its own local disk for two distinct purposes:

- **Per-invocation scratch** — the working directory for one in-progress invocation. Short-lived; removed once the invocation completes.
- **A worker-local input cache** — a content-addressed cache of files fetched from object storage, persisted across invocations on that worker. A cache hit avoids re-fetching data the worker has already pulled down (a foundational reference dataset, an intermediate output from earlier in the same pipeline run, a broadcast input shared by many mapped invocations).

When a block finishes successfully, the worker uploads each of its outputs to object storage. When a downstream block is dispatched — to that same worker or a different one — the worker resolves each input by checking its local cache first; on a miss, it fetches the file from object storage and stores it under a content-addressed key for future reuse. Because the cache is purely a performance optimization, losing it (a fresh worker, a disk failure, a manual clear) affects latency, not correctness — the data is always recoverable from object storage.

This means a worker can be added, removed, or replaced without coordinating with any other worker, and the scheduler never needs to reason about which physical machine holds which file.

## Workers and working directory setup

A **worker** receives a block assignment from the scheduler and prepares the execution environment. The setup process is:

1. **Create the working directory** with subdirectories: `inputs/`, `outputs/`, `logs/`
2. **Write `params.yaml`** containing the block's parameters from the pipeline's `args`
3. **Write `invocation.yaml`** containing metadata about the invocation (see [Blocks: invocation.yaml](/concepts/blocks/#invocationyaml) for the exact schema)
4. **Resolve inputs from the local cache.** For each declared input, the worker either finds the file already in its local cache (cache hit) or fetches it from object storage and stores it under a content-addressed key (cache miss). It then creates a symlink from `inputs/<name>/` to the cached copy. This is why the working directory is self-contained regardless of which worker produced the upstream output or which worker is running the current block.

The result is a self-contained working directory:

```
<working-directory>/
  params.yaml
  invocation.yaml
  inputs/
    image -> <local-cache>/<content-hash-of-satellite_image.tif>
    model -> <local-cache>/<content-hash-of-trained_model.pkl>
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

## Trust and integrity

Before running a block, the worker looks it up in a local **block index** -- a rebuildable cache of what's installed under `~/.spade/blocks/`, tracking each block's collection, version, language, entrypoint, and a content hash of its binary or script computed at install time. Two checks happen at dispatch:

- **Tamper detection.** The worker recomputes the hash of the block's binary or script and compares it against the value recorded at install time. If they don't match, the worker refuses to execute the block. This catches both accidental corruption and a compromised process replacing a binary underneath the worker.
- **Recall check.** A block collection version published to the registry can later be marked **recalled** (for example, after a security issue is discovered). If the worker's index entry for that version is stale, it re-checks the registry before running. A recalled version is refused, reported to the scheduler, and removed from the local install -- even if the worker already had it unpacked and had run it successfully before.

This is a runtime safety net, separate from the registry's own publish-time trust chain (screening, signed builds) -- see [Block Collections](/concepts/collections/#publishing-and-the-registry) for how a collection gets published and signed in the first place.

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
- Workers do **not** share a filesystem with each other or with the scheduler -- see [Storage model: no shared filesystem](#storage-model-no-shared-filesystem) above. Data moves between workers through object storage, not a network-attached volume.

The scheduler dispatches each block invocation as a message on a durable job queue; workers consume competing-consumer style, so work fans out across however many workers are running with no additional scheduler-side bookkeeping. From the pipeline's perspective, distributed execution behaves identically to local execution: the same dependency resolution, sandboxing, and caching rules apply. The only difference is that blocks can run on different machines, increasing total throughput.

## Error handling

Spade distinguishes two failure modes, because they call for different responses.

### Block failure

A block failure is when the block's subprocess runs to completion but **exits with a non-zero exit code**. The block ran; its logic (or its inputs) produced an error. When this happens:

1. **The pipeline halts.** The scheduler stops dispatching new blocks. Blocks that are already running are allowed to finish, but no new blocks are started.
2. **Logs are preserved.** The failed block's `logs/stdout.log` and `logs/stderr.log` are uploaded to object storage before the failure is reported, so they remain available even after the local working directory is cleaned up.
3. **Spade reports the failure.** The CLI displays which block failed, its exit code, and the path to its logs.

To debug a failure:

```bash
# Run with preserved working directories
spade run --keep-work-dir my-pipeline.yaml

# Check the failed block's logs
cat ~/.spade/pipelines/<run-id>/<block-id>/logs/stderr.log
```

Blocks that completed successfully before the failure are cached normally. When you fix the failing block and re-run the pipeline, the successful blocks are skipped (served from cache) and only the previously-failed block and its downstream dependents re-execute.

### Worker failure

A worker failure is different: the **worker process itself** crashes, its host loses power, or the network drops before it can report a result -- the block's own exit code is never the issue, because the worker never got to report one. Spade handles this transparently:

- The worker never acknowledges the job as done, so the job is automatically redelivered to another worker once a timeout elapses.
- The redelivered job runs the block fresh, with a new working directory and a new set of logs. Any logs from the failed attempt are lost along with the worker.
- The scheduler is not aware a first attempt was made -- from its perspective, the block simply took a bit longer to complete.

You will not typically see a worker failure surfaced as a pipeline error; if you notice a block invocation retried with no corresponding error in your history, this is why. Locally, with a single worker, a worker failure just means `spade run` itself was interrupted -- rerunning picks the pipeline back up using the cache, exactly as a block failure does.

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
