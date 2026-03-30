# Core Library Implementation Plan

This plan details the changes needed to bring the `core` Go module into full alignment with the Spade specifications in `../spec/`. Each phase is ordered so that later phases build on earlier ones. Within each phase, items can generally be completed in order.

---

## Phase 1: Core Types and Data Structures (`types.go`)

The existing types are incomplete and contain bugs. This phase brings the type definitions into alignment with the block, pipeline, scheduler, and worker specifications.

### Phase 1.1: Input Reference Types

- [DONE] Define an `InputRef` struct that supports both forms of pipeline input references. It must contain an optional `Block` UUID field (pointer, nil for bare references), an optional `Output` string field (the named output), and a raw `ID` UUID field (populated for bare references). This corresponds to the two input reference forms in `pipeline.md` section 5: bare invocation IDs and explicit `block` + `output` pairs.
- [DONE] Implement a custom `UnmarshalYAML` method on `InputRef` that handles both YAML forms: a plain UUID string (bare reference) and a mapping with `block` and `output` keys (explicit reference). This is required because the pipeline YAML mixes both forms in the same `inputs` list.
- [DONE] Implement a corresponding `MarshalYAML` method so that `InputRef` values round-trip correctly through YAML serialization.

### Phase 1.2: Block Manifest Types

- [DONE] Define a `BlockKind` string type with constants `BlockKindStandard`, `BlockKindMap`, and `BlockKindReduce` (values `"standard"`, `"map"`, `"reduce"`). This corresponds to the `kind` field in `blocks.md` section 3.2.
- [DONE] Define an `InputDeclaration` struct with fields `Type` (string, required), `Format` (string, optional), `Description` (string, optional), and `ItemType` (string, optional, for collections). This matches `blocks.md` section 3.3.
- [DONE] Define an `OutputDeclaration` struct with the same fields as `InputDeclaration`. Output types include `file`, `directory`, `collection`, `json`, and `expansion` (`blocks.md` section 6.1).
- [DONE] Define a `BlockManifest` struct representing a parsed `block.yaml` file. Fields: `ID` (string, e.g. `"gdal.rasterize"`), `Version` (string, semver), `Kind` (BlockKind, default `"standard"`), `Network` (bool, default `false`), `Description` (string), `Entrypoint` (string, optional override), `Inputs` (map[string]InputDeclaration), `Outputs` (map[string]OutputDeclaration). This is the full manifest defined in `blocks.md` section 3.
- [DONE] Implement a `LoadBlockManifest(path string) (BlockManifest, error)` function that reads and parses a `block.yaml` (or `blocks/<name>.yaml`) file into a `BlockManifest` struct using `gopkg.in/yaml.v3`.

### Phase 1.3: Pipeline Types

- [DONE] Update the `BlockArgs` struct (rename to `PipelineBlock` for clarity): change `Inputs` from `[]uuid.UUID` to `[]InputRef` to support both bare and explicit input references. Export the `Outputs` field (currently lowercase `outputs`). Add `yaml` struct tags matching the pipeline YAML field names (`id`, `name`, `inputs`, `args`).
- [DONE] Update the `Pipeline` struct: add a `Description` field (string, optional). Add `yaml` struct tags for all fields matching the pipeline YAML format (`id`, `name`, `version`, `description`, `blocks`).
- [DONE] Implement `LoadPipeline(path string) (Pipeline, error)` that reads and parses a pipeline YAML file into a `Pipeline` struct.
- [DONE] Implement `SavePipeline(pipeline Pipeline, path string) error` that serializes a `Pipeline` to YAML.

### Phase 1.4: Execution and Invocation Types

- [DONE] Update `BlockInvocation` to use `[]InputRef` for its `Inputs` field instead of `[]uuid.UUID`, matching the pipeline input reference model.
- [DONE] Add a `MapIndex` field (`*int`, nil when not in a map context) to `BlockInvocation` to support the `<block_id>.<index>` invocation ID scheme described in `scheduler.md`.
- [DONE] Add an `InvocationID() string` method to `BlockInvocation` that returns the invocation ID string. For non-mapped blocks this is just the UUID string; for mapped blocks it is `<UUID>.<MapIndex>`.
- [DONE] Fix the `ExecutionStatus` constants to properly use the `ExecutionStatus` type (currently they are untyped string constants; they should be `ExecutionStatus("waiting")`, etc.).
- [DONE] Update `BlockInvocationResult`: add an `Expansion *ExpansionManifest` field (populated when a map block completes) and an `Error` string field for error messages.

### Phase 1.5: Expansion Manifest Types

- [DONE] Define an `ExpansionItem` struct with `Path` (string, relative file path) and `Key` (string, human-readable identifier). This matches the expansion manifest format in `blocks.md` section 6.2 and `scheduler.md`.
- [DONE] Define an `ExpansionManifest` struct with `Items []ExpansionItem`. This is the parsed form of the `expansion.yaml` file written by map blocks.
- [DONE] Implement `LoadExpansionManifest(path string) (ExpansionManifest, error)` that reads and parses an `expansion.yaml` file.

### Phase 1.6: Invocation Metadata Type

- [DONE] Define an `InvocationMetadata` struct matching the `invocation.yaml` format from `blocks.md` section 7: nested `Block` struct (ID string, Version string), `InvocationID` string, and `Inputs` map (name -> struct with `Path` and `Hash`).
- [DONE] Implement `WriteInvocationMetadata(meta InvocationMetadata, dir string) error` that serializes the metadata to `invocation.yaml` in the given working directory.

### Phase 1.7: Block Registry Types

- [DONE] Define a `BlockRegistryEntry` struct with GORM model tags for the SQLite block registry. Fields: `ID` (uint, primary key), `CollectionName` (string), `CollectionVersion` (string), `BlockName` (string), `BlockID` (string, e.g. `"gdal.rasterize"`), `Language` (string), `Entrypoint` (string), `InstalledPath` (string), `ContentHash` (string), `Kind` (string), `Network` (bool), `ManifestJSON` (string, serialized block manifest for quick access). This corresponds to `worker.md` section "Block Registry".
- [DONE] Define a `CollectionLanguage` string type with constants for `Rust`, `Go`, `Python`, `TypeScript`, and `R`. This maps to the language detection table in `blocks.md` section 2.1.

### Phase 1.8: Worker Communication Types

- [DONE] Define a `WorkerAssignment` struct representing a block execution assignment sent from scheduler to worker. Fields: `InvocationID` (string), `BlockName` (string), `PipelineID` (uuid.UUID), `WorkDir` (string), `Args` (map[string]any), `Inputs` ([]InputRef).
- [DONE] Define a `WorkerResult` struct representing the completion response from worker to scheduler. Fields: `InvocationID` (string), `PipelineID` (uuid.UUID), `Status` (ExecutionStatus), `Error` (string, if failed), `Expansion` (*ExpansionManifest, if map block), `OutputHashes` (map[string]string).

### Phase 1.9: Cleanup

- [DONE] Remove the `go-landlock` and `libcap/psx` dependencies from `go.mod` since the spec requires using `isolate` for sandboxing, not go-landlock (`worker.md` section "Security").
- [DONE] Add `gopkg.in/yaml.v3` dependency to `go.mod` for YAML parsing (currently missing).
- [DONE] Remove the unused `Block` struct (it duplicates `PipelineBlock` / `BlockManifest` without adding value). Update any references.
- [DONE] Remove the unused `ExecutionPlan` struct (the scheduler tracks execution state directly via pending/completed/executable block maps, not through a separate plan object).

---

## Phase 2: Pipeline Parsing and Validation (`pipeline.go`, new file)

This phase implements pipeline loading, dependency graph construction, and the full validation suite described in `pipeline.md` section 7.

### Phase 2.1: Dependency Graph

- [DONE] Define a `DependencyGraph` struct that holds the adjacency list (map from block UUID to list of dependent block UUIDs) and reverse adjacency list (map from block UUID to list of dependency UUIDs). This represents the DAG described in `pipeline.md` section 6.
- [DONE] Implement `BuildDependencyGraph(pipeline Pipeline) (DependencyGraph, error)` that constructs the graph from the pipeline's block input references. Both bare and explicit references create edges.
- [DONE] Implement `(g *DependencyGraph) TopologicalSort() ([]uuid.UUID, error)` using Kahn's algorithm. Returns an error if a cycle is detected. This determines valid execution order per `pipeline.md` section 6.
- [DONE] Implement `(g *DependencyGraph) SourceBlocks() []uuid.UUID` that returns all blocks with no incoming edges (empty `inputs`). These can execute immediately per `pipeline.md` section 6.
- [DONE] Implement `(g *DependencyGraph) DownstreamBlocks(id uuid.UUID) []uuid.UUID` that returns all blocks that directly depend on the given block.

### Phase 2.2: Input Resolution Algorithm

- [DONE] Implement `ResolveInputs(block PipelineBlock, dependencies map[uuid.UUID]BlockManifest, currentManifest BlockManifest) (map[string]ResolvedInput, error)`. This implements the 4-step resolution algorithm from `pipeline.md` section 5.4: (1) resolve explicit references by matching `block`+`output` to the named output of the dependency, (2) resolve bare references by type-matching remaining unmatched outputs to unmatched inputs, (3) reject if ambiguous, (4) reject if incomplete.
- [DONE] Define a `ResolvedInput` struct with fields: `InputName` (string, the name on the current block), `SourceBlockID` (uuid.UUID), `SourceOutputName` (string), and `SourceOutputDecl` (OutputDeclaration).

### Phase 2.3: Pipeline Validation

- [DONE] Implement `ValidatePipeline(pipeline Pipeline, manifests map[string]BlockManifest) []error` that performs all validation checks from `pipeline.md` section 7:
  1. All block `id` values are unique within the pipeline.
  2. All invocation IDs referenced in `inputs` exist in the pipeline's block list.
  3. All `name` values refer to known block types (checked against provided manifests).
  4. The dependency graph is acyclic (uses `TopologicalSort`).
  5. Input/output type compatibility between connected blocks (uses `ResolveInputs`).
  6. Named output references match outputs declared in the dependency block's manifest.
  7. All required `args` for each block are present (checked against manifest input declarations for scalar types).
- [DONE] Implement map/reduce-specific validation from `pipeline.md` section 8.3:
  1. Every `kind: map` block is eventually followed by a `kind: reduce` block in the dependency graph.
  2. No nested maps (a map context must be closed by a reduce before another map begins).
  3. Map blocks output the `expansion` type.
  4. Reduce blocks accept a `collection` input.

---

## Phase 3: Block Management (`block.go`)

This phase implements the filesystem operations, block registry, and block lookup logic described in `worker.md`.

### Phase 3.1: Block Directory and Filesystem Setup

- [DONE] Fix `CreateBlockDirectory` to create the full directory structure per `worker.md` section "File System": the main invocation directory, plus `inputs/`, `outputs/`, and `logs/` subdirectories. Use `0755` permissions for directories (not `os.ModeDir` which is incorrect as a permission).
- [DONE] Implement `WriteParamsYAML(args map[string]any, workDir string) error` that serializes the block's `args` to `params.yaml` in the working directory using `gopkg.in/yaml.v3`. This replaces the stub `WriteArgs` function in `executor.go`. Per `blocks.md` section 5.4, all scalar inputs are provided via `params.yaml`.
- [DONE] Implement `SetupInputSymlinks(workDir string, resolvedInputs map[string]ResolvedInput, pipelineDir string) error` that creates symlinks in `<workDir>/inputs/<input_name>/` pointing to the output files from dependency blocks. For each resolved input, the symlink target is `<pipelineDir>/<source_block_id>/outputs/<source_output_name>/`. This implements the symlinking described in `worker.md` section "File System".
- [DONE] Implement `SetupBroadcastInputs(workDir string, nonMappedInputs map[string]ResolvedInput, pipelineDir string) error` for map context: symlinks non-mapped dependency outputs into every mapped invocation's `inputs/` directory. Per `scheduler.md` section "Broadcasting Non-Mapped Inputs".
- [DONE] Fix the typo: rename `CollectOuptuts` to `CollectOutputs`.
- [DONE] Implement `CollectOutputs(workDir string) (map[string]string, error)` that scans the `outputs/` directory, computes content hashes (SHA-256) for each output file/directory, and returns a map of output name to hash. Per `blocks.md` section 9, output hashes are used for caching.

### Phase 3.2: Block Registry (`registry.go`, new file)

- [DONE] Implement `OpenRegistry(dbPath string) (*BlockRegistry, error)` that opens (or creates) the SQLite database at the given path with GORM, enables WAL mode, sets file permissions to `0600`, and auto-migrates the `BlockRegistryEntry` schema. Per `worker.md` section "Block Registry - Implementation details".
- [DONE] Implement `(r *BlockRegistry) RegisterBlock(entry BlockRegistryEntry) error` that inserts or updates a block entry in the registry.
- [DONE] Implement `(r *BlockRegistry) LookupBlock(name string, version string) (*BlockRegistryEntry, error)` that queries the registry by block name (e.g. `"gdal.rasterize"`) and optional version. If version is empty, returns the latest version. Per `worker.md` section "Block Lookup".
- [DONE] Implement `(r *BlockRegistry) ListBlocks() ([]BlockRegistryEntry, error)` that returns all registered blocks.
- [DONE] Implement `(r *BlockRegistry) DeleteCollection(name string, version string) error` that removes all blocks for a given collection and version.
- [DONE] Implement `(r *BlockRegistry) RebuildFromFilesystem(blocksDir string) error` that scans `~/.spade/blocks/`, re-reads all `blocks/*.yaml` manifests, recomputes content hashes, and repopulates the database. Per `worker.md` section "Rebuilding".

### Phase 3.3: Block Lookup and Integrity (`registry.go`)

- [DONE] Implement `ComputeContentHash(path string) (string, error)` that computes a SHA-256 hash of a file or, for directories, a deterministic hash of all files within. This is the hash compared at execution time per `worker.md` section "Integrity verification".
- [DONE] Implement `(r *BlockRegistry) VerifyBlock(entry BlockRegistryEntry) error` that recomputes the content hash and compares it against the stored hash. Returns an error if they do not match. Per `worker.md`: "If the hashes do not match, the worker refuses to execute the block."
- [DONE] Implement `DetectLanguage(repoRoot string) (CollectionLanguage, error)` that determines the collection language by checking for `Cargo.toml` (Rust), `go.mod` (Go), `pyproject.toml` (Python), `package.json` (TypeScript), or defaults to R. Per `blocks.md` section 2.1.
- [DONE] Implement `DiscoverBlocks(repoRoot string) ([]string, error)` that scans the `blocks/` directory for `*.yaml` files and returns the list of block manifest paths. Per `blocks.md` section 2.1: "The CLI discovers everything by scanning the repository."

### Phase 3.4: Entrypoint Resolution

- [DONE] Implement `ResolveEntrypoint(entry BlockRegistryEntry) (string, []string, error)` that returns the executable path and arguments for running a block based on its language. Per `worker.md` section "Execution":
  - Rust/Go: `<collection_binary>` with block name as subcommand arg.
  - TypeScript (Bun): `<bundled_binary>` with block name as subcommand arg.
  - Python: `uv run <entrypoint>`.
  - R: `Rscript <entrypoint>`.
  The function returns `(executablePath string, args []string, err error)`.

---

## Phase 4: Scheduler (`scheduler.go`)

This phase fixes bugs in the existing scheduler and implements the full map/reduce mechanics from `scheduler.md`.

### Phase 4.1: Fix SinglePipelineScheduler Bugs

- [DONE] Fix the dependency-check bug in `Update()`: the current code checks `s.PendingBlocks[item]` to determine if a block is executable, but this is backwards -- a block is executable when all its dependencies are in `CompletedBlocks`, not when they are still in `PendingBlocks`. Change the check to: for each input dependency, verify it exists in `s.CompletedBlocks`. A block is executable when ALL dependencies are completed.
- [DONE] Fix `AddPipeline()` to identify source blocks (blocks with empty `Inputs`) and add them to `ExecutableBlocks` immediately, instead of only adding them to `PendingBlocks`. Without this, the scheduler can never start because `Update()` is only called after a block completes, creating a deadlock for the first block.
- [DONE] Fix `Update()` to remove blocks from `PendingBlocks` when they become executable (move to `ExecutableBlocks`), preventing them from being re-evaluated on every subsequent update.

### Phase 4.2: Map Context Tracking

- [DONE] Add a `MapContexts` field to `SinglePipelineScheduler`: `map[uuid.UUID]*MapContext`. Each `MapContext` tracks: `MapBlockID` (uuid.UUID), `ExpansionItems` ([]ExpansionItem), `MappedBlockIDs` (set of block UUIDs that are in this map context), and `ReduceBlockID` (uuid.UUID).
- [DONE] Implement `(s *SinglePipelineScheduler) IdentifyMapContexts() error` that walks the dependency graph from each `kind: map` block forward until a `kind: reduce` block is reached. All blocks between the map and reduce (inclusive) are part of the map context. This implements the propagation rule from `scheduler.md` section "Map Context Propagation".
- [DONE] Store the `BlockManifest` for each block in the scheduler (add a `Manifests map[string]BlockManifest` field or accept manifests in `AddPipeline`) so the scheduler can check `Kind` when processing blocks.

### Phase 4.3: Implement HandleMap

- [DONE] Implement `HandleMap` to process map block completion. When a map block completes with an expansion manifest:
  1. Parse the expansion manifest items list from the result.
  2. For the immediate downstream block, create N `BlockInvocation` entries (one per expansion item), with `MapIndex` set to `0, 1, 2, ...` and invocation IDs following the `<block_id>.<index>` scheme.
  3. For every subsequent block in the map context (identified in Phase 4.2), also create N invocations with matching indices.
  4. Wire the dependencies: invocation `<B>.<i>` depends on `<A>.<i>` (same index through the chain). Per `scheduler.md`: "ccc.2 always feeds into ddd.2".
  5. For non-mapped dependencies (broadcast inputs), each mapped invocation depends on the single non-mapped block. Per `scheduler.md` section "Broadcasting Non-Mapped Inputs".
  6. Add all newly created invocations to `PendingBlocks`. Identify any that are immediately executable (their mapped dependency from the map block is already complete).

### Phase 4.4: Implement HandleReduce

- [DONE] Implement `HandleReduce` to detect when all N mapped invocations of the last block in a map context have completed. When they have:
  1. Create a single `BlockInvocation` for the reduce block.
  2. The reduce block's inputs reference all N completed invocations' outputs as a collection.
  3. Add the reduce block to `ExecutableBlocks`.
  4. Clean up the `MapContext` entry.

### Phase 4.5: Fix MultiTenantScheduler

- [DONE] Fix `MultiTenantScheduler.Update()`: the method currently retrieves the scheduler by value from the map (`scheduler, ok := s.Schedulers[result.PipelineId]`), which means updates are applied to a copy, not the original. Change `Schedulers` to `map[uuid.UUID]*SinglePipelineScheduler` (pointer values) so mutations are reflected in the map.
- [DONE] Add worker tracking to `MultiTenantScheduler.Next()`: record which worker received which block invocation in `CurrentExecutions` so the scheduler can track worker utilization and handle worker failures.
- [DONE] Implement `(s *MultiTenantScheduler) AddWorker(worker Worker) error` and `(s *MultiTenantScheduler) RemoveWorker(id uuid.UUID) error` for managing the worker pool.
- [DONE] Implement fair scheduling in `Next()`: instead of iterating pipelines in map order (which is nondeterministic and unfair), use a round-robin or priority-based approach across pipelines. This ensures fair execution per `scheduler.md` section "Multiple Instance Scheduler".

### Phase 4.6: Error Handling

- [DONE] Implement pipeline halt on block failure: when `Update()` receives an `ExecutionStatusError`, cancel the pipeline (clear pending and executable blocks) but preserve completed block results for debugging. Per `scheduler.md` section "Error Handling": "the scheduler halts the pipeline that the failed block belongs to."
- [DONE] For `MultiTenantScheduler`, ensure that a failure in one pipeline does not affect other pipelines. Per `scheduler.md`: "Other concurrently running pipelines are not affected."

---

## Phase 5: Executor (`executor.go`)

This phase implements the full block execution lifecycle described in `worker.md`.

### Phase 5.1: Block Execution Setup

- [DONE] Refactor `Execute()` to accept additional context: the `BlockManifest` for the block being executed, the `BlockRegistryEntry` for locating the binary, and the pipeline directory for resolving input symlinks.
- [DONE] In `Execute()`, call `VerifyBlock()` to check the content hash before execution. If verification fails, return an error immediately. Per `worker.md` section "Integrity verification".
- [DONE] In `Execute()`, call `CreateBlockDirectory()` to set up the full directory structure (`inputs/`, `outputs/`, `logs/`).
- [DONE] In `Execute()`, call `WriteParamsYAML()` to write the block's args to `params.yaml`.
- [DONE] In `Execute()`, call `WriteInvocationMetadata()` to write `invocation.yaml` with block info and input hashes.
- [DONE] In `Execute()`, call `SetupInputSymlinks()` to create symlinks from the block's `inputs/` to its dependencies' `outputs/`.

### Phase 5.2: Subprocess Execution with Isolate

- [DONE] Implement `RunBlockSubprocess(execPath string, args []string, workDir string, manifest BlockManifest) (int, error)` that executes the block as a subprocess using `os/exec`. The command should be wrapped with the `isolate` sandbox. The working directory is set to the block's invocation directory. Per `worker.md` section "Security": use Ubuntu `isolate` package.
- [DONE] Configure `isolate` to restrict the block to its invocation working directory. Per `worker.md`: "Restrict the block to its invocation working directory."
- [DONE] Configure `isolate` to allow execution of required system binaries (e.g. GDAL, Apache Arrow) by providing appropriate read-only mounts.
- [DONE] Configure `isolate` to enforce memory and CPU time limits.
- [DONE] Configure `isolate` to block network access by default. If the block manifest has `Network: true`, configure `isolate` to allow network access. Per `worker.md`: "Blocks do not have network access by default."
- [DONE] Capture `stdout` and `stderr` from the subprocess, writing them to `logs/stdout.log` and `logs/stderr.log` in the block's working directory. Per `worker.md` section "Logging".

### Phase 5.3: Post-Execution Processing

- [DONE] After subprocess completion, check the exit code. If non-zero, return a `BlockInvocationResult` with `ExecutionStatusError` and populate the `Error` field with relevant log output. Per `worker.md` section "Error Handling": "If a block exits with a non-zero exit code, the worker reports the failure."
- [DONE] If the block is `kind: map`, read the expansion manifest from `outputs/manifest/expansion.yaml` (or the appropriate output name) using `LoadExpansionManifest()`. Include the parsed manifest in the `BlockInvocationResult`. Per `worker.md` section "Map Block Handling".
- [DONE] Call `CollectOutputs()` to hash and record the block's outputs. Include output hashes in the result for caching.
- [DONE] Remove the `WriteArgs` stub function (replaced by `WriteParamsYAML` in Phase 3.1).

---

## Phase 6: Caching (`cache.go`, new file)

This phase implements the caching system described in `blocks.md` section 9 and `pipeline.md` section 10.

### Phase 6.1: Cache Key Computation

- [DONE] Implement `ComputeCacheKey(blockID string, blockVersion string, inputHashes map[string]string, params map[string]any) (string, error)` that computes a deterministic cache key from the block ID, version, input content hashes, and serialized params.yaml. Per `blocks.md` section 9: "Cache keys are derived from block ID + version, input content hashes, params.yaml, runtime environment hash."
- [DONE] Implement `ComputeRuntimeHash() (string, error)` that hashes the relevant runtime environment (Go version, OS, architecture) for inclusion in the cache key.

### Phase 6.2: Cache Storage

- [DONE] Implement `CacheLookup(cacheKey string, cacheDir string) (string, bool)` that checks if cached outputs exist for the given key. Returns the path to the cached outputs directory and whether the cache hit was successful.
- [DONE] Implement `CacheStore(cacheKey string, outputsDir string, cacheDir string) error` that copies the block's outputs to the cache directory keyed by the cache key.
- [DONE] Implement `CacheRestore(cacheKey string, targetDir string, cacheDir string) error` that restores cached outputs into a block's working directory, skipping re-execution.

---

## Phase 7: Tests

Tests should validate each layer of the implementation. Use the standard Go testing package.

### Phase 7.1: Type and Parsing Tests (`types_test.go`)

- [DONE] Test `InputRef` YAML unmarshaling: verify that bare UUID strings parse into `InputRef` with `ID` populated and `Block`/`Output` nil. Verify that `{block: <uuid>, output: "name"}` mappings parse correctly.
- [DONE] Test `InputRef` YAML marshaling round-trip for both forms.
- [DONE] Test `LoadBlockManifest`: parse a sample `block.yaml` with all fields (kind, network, inputs, outputs) and verify the resulting struct.
- [DONE] Test `LoadPipeline`: parse the example pipeline YAML from `pipeline.md` section 2 and verify all fields including mixed input reference forms.
- [DONE] Test `LoadExpansionManifest`: parse a sample `expansion.yaml` and verify item paths and keys.

### Phase 7.2: Pipeline Validation Tests (`pipeline_test.go`)

- [DONE] Test `BuildDependencyGraph` with a simple 3-block linear pipeline (A -> B -> C) and verify edges.
- [DONE] Test `TopologicalSort` returns a valid ordering for a diamond DAG (A -> B, A -> C, B -> D, C -> D).
- [DONE] Test `TopologicalSort` returns an error for a pipeline with a cycle (A -> B -> C -> A).
- [DONE] Test `SourceBlocks` correctly identifies blocks with no inputs.
- [DONE] Test `ResolveInputs` with bare references and unambiguous type matching.
- [DONE] Test `ResolveInputs` with explicit references that override type matching.
- [DONE] Test `ResolveInputs` rejects ambiguous bare references (two outputs of the same type for one input of that type).
- [DONE] Test `ResolveInputs` rejects incomplete references (an input with no match).
- [DONE] Test `ValidatePipeline` catches duplicate block IDs.
- [DONE] Test `ValidatePipeline` catches references to non-existent block IDs.
- [DONE] Test `ValidatePipeline` catches references to unknown block types (names not in manifests).
- [DONE] Test map/reduce validation: map without eventual reduce is rejected.
- [DONE] Test map/reduce validation: nested maps (map inside map without reduce) is rejected.
- [DONE] Test map/reduce validation: map block must output `expansion` type.

### Phase 7.3: Block Filesystem Tests (`block_test.go`)

- [DONE] Test `CreateBlockDirectory` creates the main directory plus `inputs/`, `outputs/`, and `logs/` subdirectories.
- [DONE] Test `WriteParamsYAML` writes correct YAML content for various argument types (strings, numbers, booleans, nested maps).
- [DONE] Test `SetupInputSymlinks` creates correct symlinks pointing to dependency output directories.
- [DONE] Test `CollectOutputs` correctly hashes output files and returns the expected map.
- [DONE] Test `WriteInvocationMetadata` writes correct `invocation.yaml` content.

### Phase 7.4: Block Registry Tests (`registry_test.go`)

- [DONE] Test `OpenRegistry` creates a new SQLite database with correct permissions (0600) and WAL mode.
- [DONE] Test `RegisterBlock` and `LookupBlock` round-trip: register a block, look it up by name, verify all fields.
- [DONE] Test `LookupBlock` with version: register two versions of the same block, look up each by version.
- [DONE] Test `VerifyBlock` succeeds when hash matches and fails when it does not.
- [DONE] Test `DetectLanguage` for each supported language (check for Cargo.toml, go.mod, pyproject.toml, package.json, R default).
- [DONE] Test `DiscoverBlocks` finds all `*.yaml` files in a `blocks/` directory.

### Phase 7.5: Scheduler Tests (`scheduler_test.go`)

- [DONE] Test `SinglePipelineScheduler` with a simple linear pipeline (A -> B -> C): verify source block A is immediately executable, B becomes executable after A completes, C after B.
- [DONE] Test `SinglePipelineScheduler` with parallel blocks: pipeline with A -> B, A -> C (B and C independent). After A completes, both B and C should be executable.
- [DONE] Test `SinglePipelineScheduler` with diamond DAG: A -> B, A -> C, B -> D, C -> D. D should only become executable after both B and C complete.
- [DONE] Test `SinglePipelineScheduler.Update` with `ExecutionStatusError` halts the pipeline (clears pending blocks).
- [DONE] Test `HandleMap`: verify that N invocations are created for downstream blocks with correct MapIndex values and invocation IDs.
- [DONE] Test `HandleReduce`: verify that the reduce block becomes executable only after all N mapped invocations complete.
- [DONE] Test map context propagation: in a chain map -> B -> C -> reduce, verify both B and C get N invocations with matching indices.
- [DONE] Test `MultiTenantScheduler` with two pipelines: verify both get fair scheduling (neither is starved).
- [DONE] Test `MultiTenantScheduler` error isolation: failing one pipeline does not affect the other.

### Phase 7.6: Executor Tests (`executor_test.go`)

- [DONE] Test the full `Execute` flow with a mock block (a simple shell script that reads params.yaml and writes an output file). Verify directory structure, params.yaml content, and output collection.
- [DONE] Test that `Execute` returns `ExecutionStatusError` when the subprocess exits with non-zero code.
- [DONE] Test that `Execute` correctly reads the expansion manifest for a mock map block.
- [DONE] Test stdout/stderr capture: verify that subprocess output is written to `logs/stdout.log` and `logs/stderr.log`.

### Phase 7.7: Cache Tests (`cache_test.go`)

- [DONE] Test `ComputeCacheKey` produces the same key for identical inputs and different keys for different inputs.
- [DONE] Test `CacheStore` and `CacheLookup` round-trip: store outputs, verify lookup finds them.
- [DONE] Test `CacheRestore` correctly copies cached outputs to a new working directory.
- [DONE] Test cache miss: `CacheLookup` returns false for a key that has not been stored.

---

## Phase 8: Module Cleanup and Integration

### Phase 8.1: Module Dependencies

- [DONE] Run `go mod tidy` to clean up `go.sum` and remove unused transitive dependencies after removing `go-landlock` and `libcap/psx`.
- [DONE] Verify that all new dependencies (`gopkg.in/yaml.v3`, etc.) are properly recorded in `go.mod`.
- [DONE] Ensure the module compiles with `go build ./...` and all tests pass with `go test ./...`.

### Phase 8.2: Exported API Surface

- [DONE] Review all exported types and functions to ensure a clean, minimal public API. The core module is consumed by the CLI, cloud resources, and plugins per `PROMPT.md`. Key exports should include:
  - Types: `Pipeline`, `PipelineBlock`, `InputRef`, `BlockManifest`, `InputDeclaration`, `OutputDeclaration`, `BlockKind`, `BlockInvocation`, `BlockInvocationResult`, `ExecutionStatus`, `ExpansionManifest`, `ExpansionItem`, `Worker`, `BlockRegistryEntry`, `CollectionLanguage`
  - Pipeline: `LoadPipeline`, `SavePipeline`, `ValidatePipeline`, `BuildDependencyGraph`, `ResolveInputs`
  - Block: `LoadBlockManifest`, `CreateBlockDirectory`, `WriteParamsYAML`, `SetupInputSymlinks`, `CollectOutputs`
  - Registry: `OpenRegistry` (and methods on `*BlockRegistry`)
  - Scheduler: `SinglePipelineScheduler`, `MultiTenantScheduler`, `NewSchedulerForPipeline`
  - Executor: `Execute`
  - Cache: `ComputeCacheKey`, `CacheLookup`, `CacheStore`, `CacheRestore`
- [DONE] Ensure unexported helpers are not leaking through the API. Internal helpers like `ComputeContentHash`, `DetectLanguage`, `DiscoverBlocks` may be exported if the CLI needs them; otherwise keep them unexported.

### Phase 8.3: File Organization

- [DONE] Organize source files by concern:
  - `types.go` - All type definitions and constants
  - `pipeline.go` - Pipeline parsing, validation, dependency graph, input resolution
  - `block.go` - Block manifest loading, directory setup, filesystem operations
  - `registry.go` - Block registry (SQLite), block lookup, integrity verification
  - `scheduler.go` - SinglePipelineScheduler, MultiTenantScheduler, map/reduce handling
  - `executor.go` - Block execution, subprocess management, isolate integration
  - `cache.go` - Caching logic
  - Test files mirror source files: `types_test.go`, `pipeline_test.go`, `block_test.go`, `registry_test.go`, `scheduler_test.go`, `executor_test.go`, `cache_test.go`
