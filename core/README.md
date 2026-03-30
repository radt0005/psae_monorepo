# core

The `core` Go module provides the shared types, scheduling logic, pipeline validation, block management, and execution infrastructure for the Spade data processing system. It is consumed by the CLI, the worker binary, and cloud-side components.

## Package layout

| File | Responsibility |
|---|---|
| `types.go` | All type definitions: pipelines, blocks, manifests, input references, execution status, worker communication, expansion manifests, invocation metadata, registry entries |
| `pipeline.go` | Dependency graph construction, topological sort, input resolution algorithm, pipeline validation (including map/reduce rules) |
| `block.go` | Block directory setup, `params.yaml` writing, input symlinking, output collection and hashing, language detection, block discovery, entrypoint resolution |
| `registry.go` | SQLite block registry (via GORM): open, register, lookup, list, delete, rebuild, integrity verification |
| `scheduler.go` | `SinglePipelineScheduler` and `MultiTenantScheduler` with map/reduce fan-out, fair round-robin scheduling, and error-halting semantics |
| `executor.go` | Full block execution lifecycle: integrity check, directory setup, params/metadata writing, subprocess execution via `isolate`, output collection |
| `cache.go` | Deterministic cache key computation, cache store/lookup/restore |

## Dependencies

| Module | Purpose |
|---|---|
| `github.com/google/uuid` | UUIDv7 identifiers for pipelines, blocks, and invocations |
| `gopkg.in/yaml.v3` | YAML parsing/serialization for manifests, pipelines, params, and expansion files |
| `gorm.io/gorm` + `gorm.io/driver/sqlite` | SQLite block registry with WAL mode |

## Key concepts

### Pipelines

A `Pipeline` is a YAML-defined DAG of `PipelineBlock` entries. Each block references its dependencies via `InputRef` values, which support two forms:

- **Bare reference**: a UUID pointing to another block's invocation ID (outputs are resolved by type matching)
- **Explicit reference**: a `block` UUID + `output` name pair (directly names the dependency output)

```go
p, err := core.LoadPipeline("pipeline.yaml")
errs := core.ValidatePipeline(p, manifests)
graph, err := core.BuildDependencyGraph(p)
order, err := graph.TopologicalSort()
```

### Block manifests

A `BlockManifest` describes a block's identity, kind (`standard`, `map`, `reduce`), inputs, outputs, and runtime requirements. Manifests are loaded from `blocks/*.yaml` within a collection repository.

```go
m, err := core.LoadBlockManifest("blocks/rasterize.yaml")
```

### Scheduling

`SinglePipelineScheduler` tracks pending, executable, and completed blocks for one pipeline. It handles map expansion (creating N invocations per expansion item) and reduce aggregation. `MultiTenantScheduler` wraps multiple single-pipeline schedulers with fair round-robin dispatch and per-pipeline error isolation.

```go
s := core.NewSchedulerForPipeline(pipeline)
inv, done, err := s.Next()
s.Update(result)
```

### Block registry

The `BlockRegistry` is a SQLite index over installed block collections at `~/.spade/blocks/`. It caches block metadata for fast lookup and verifies content hashes before execution to detect post-install tampering.

```go
reg, err := core.OpenRegistry("/path/to/registry.db")
defer reg.Close()
reg.RegisterBlock(entry)
entry, err := reg.LookupBlock("gdal.rasterize", "1.0.0")
reg.VerifyBlock(*entry)
```

### Execution

`Execute` runs a block through the full lifecycle: integrity verification, directory setup (`inputs/`, `outputs/`, `logs/`), `params.yaml` and `invocation.yaml` writing, subprocess execution via the `isolate` sandbox, stdout/stderr capture, and output hash collection.

```go
result, err := core.Execute(invocation, pipelineDir, manifest, registryEntry, registry)
```

### Caching

Cache keys are computed deterministically from block identity, input content hashes, params, and runtime environment. Outputs are stored and restored by key.

```go
key, err := core.ComputeCacheKey(blockID, version, inputHashes, params)
path, hit := core.CacheLookup(key, cacheDir)
core.CacheStore(key, outputsDir, cacheDir)
core.CacheRestore(key, targetDir, cacheDir)
```

## Testing

```bash
go test ./...
```

Tests cover type serialization round-trips, dependency graph construction, topological sort (including cycle detection), input resolution (bare, explicit, ambiguous, incomplete), pipeline validation, block directory/filesystem operations, the SQLite registry, scheduler state transitions (linear, parallel, diamond DAGs, error halting, map/reduce), executor lifecycle, and cache key determinism with store/restore round-trips.

## Security model

Blocks are sandboxed at runtime using `isolate`, which restricts filesystem access to the invocation working directory, enforces memory/CPU limits, and blocks network access unless the block manifest declares `network: true`. The worker process itself remains unsandboxed to manage symlinks, the registry, and scheduler communication.
