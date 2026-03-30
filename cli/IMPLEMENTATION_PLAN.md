# CLI Implementation Plan

This plan describes the implementation of the `spade` CLI tool. The CLI is built with Go, Cobra, Viper, and BubbleTea. It uses the core library (`../core/`) for scheduling, execution, validation, caching, and registry management.

---

## Phase 1: Project Setup & Module Configuration

- [DONE] Add the `core` library as a local dependency in `go.mod` using a `replace` directive: `replace core => ../core` and add `require core v0.0.0`.
- [DONE] Add BubbleTea and Lip Gloss dependencies: `github.com/charmbracelet/bubbletea` and `github.com/charmbracelet/lipgloss`.
- [DONE] Add `github.com/google/uuid` as a direct dependency (needed for pipeline/block IDs).
- [DONE] Add `gopkg.in/yaml.v3` as a direct dependency (needed for YAML generation in `init` and `add` commands).
- [DONE] Run `go mod tidy` to resolve all transitive dependencies.
- [DONE] Update `cmd/root.go`: change the root command `Short` to `"Spade - A geospatial data processing CLI"` and `Long` to a description of the CLI's purpose (local block/pipeline development, installation, and execution).
- [DONE] Remove the unused `toggle` flag from `cmd/root.go`.

---

## Phase 2: `spade setup` Command

Creates the `~/.spade/` directory structure and initializes the block registry.

- [DONE] Create `cmd/setup.go` defining a `setupCmd` cobra command with `Use: "setup"`, `Short: "Set up the Spade system on the local machine"`.
- [DONE] In the `Run` function, create the following directories (using `os.MkdirAll`):
  - `~/.spade/`
  - `~/.spade/blocks/`
  - `~/.spade/cache/`
  - `~/.spade/pipelines/` (working directory for pipeline runs)
- [DONE] Open (or create) the block registry database at `~/.spade/registry.db` using `core.OpenRegistry`. Close it after successful creation.
- [DONE] If `--rebuild-index` flag is passed, call `registry.RebuildFromFilesystem(~/.spade/blocks/)` to rebuild the registry from the filesystem.
- [DONE] Print a success message listing the created directories.
- [DONE] Register `setupCmd` with `rootCmd` in `init()`.

### Tests (Phase 2)
- [DONE] `cmd/setup_test.go`: Test that `setup` creates the expected directory structure under a temporary home directory (override `$HOME` or accept a `--spade-dir` flag for testability).
- [DONE] Test that running `setup` twice is idempotent (no errors on second run).
- [DONE] Test that `--rebuild-index` opens the registry and calls rebuild.

---

## Phase 3: `spade init` Command

Scaffolds a new block collection in the current directory.

- [DONE] Create `cmd/init.go` defining an `initCmd` cobra command with `Use: "init"`, `Short: "Create a new block collection in the current directory"`.
- [DONE] Implement an interactive language selection prompt using BubbleTea. Present 5 options: Rust, Go, Python, TypeScript, R.
- [DONE] Alternatively, accept a `--language` / `-l` flag to skip the interactive prompt (useful for scripting/testing).
- [DONE] Based on the selected language, scaffold the following project structures:

  **Rust**:
  - [DONE] Create `Cargo.toml` with a template containing `[package]` name (derived from current directory name), version `"0.1.0"`, and edition `"2021"`.
  - [DONE] Create `src/lib.rs` with an empty module file.
  - [DONE] Create `blocks/` directory (empty).

  **Go**:
  - [DONE] Create `go.mod` with module name derived from current directory name and current Go version.
  - [DONE] Create `main.go` with a minimal `package main` and `func main()` stub.
  - [DONE] Create `blocks/` directory (empty).

  **Python**:
  - [DONE] Create `pyproject.toml` with `[project]` name (derived from directory name, underscored), version `"0.1.0"`, and `requires-python`.
  - [DONE] Create `src/<package_name>/` directory with an empty `__init__.py`.
  - [DONE] Create `blocks/` directory (empty).

  **TypeScript**:
  - [DONE] Create `package.json` with name (derived from directory name), version `"0.1.0"`, and a `"main"` field.
  - [DONE] Create `src/` directory (empty).
  - [DONE] Create `blocks/` directory (empty).

  **R**:
  - [DONE] Create `renv.lock` with a minimal valid JSON structure.
  - [DONE] Create `R/` directory (empty).
  - [DONE] Create `blocks/` directory (empty).

- [DONE] Print a summary of created files and directories.
- [DONE] Register `initCmd` with `rootCmd` in `init()`.

### Tests (Phase 3)
- [DONE] `cmd/init_test.go`: For each language, test that `init` in a temp directory creates the expected file structure.
- [DONE] Test that `--language` flag bypasses interactive selection.
- [DONE] Test that running `init` in a directory with existing files prints an error or warning (prevent accidental overwrite).

---

## Phase 4: `spade add <name>` Command

Adds a new block to the current collection.

- [DONE] Create `cmd/add.go` defining an `addCmd` cobra command with `Use: "add <name>"`, `Short: "Add a new block to the current collection"`, `Args: cobra.ExactArgs(1)`.
- [DONE] Detect the collection language using `core.DetectLanguage(".")`.
- [DONE] Create a block manifest template at `blocks/<name>.yaml` with the following fields pre-populated:
  ```yaml
  id: <directory_name>.<name>
  version: "0.1.0"
  kind: standard
  network: false
  description: ""
  entrypoint: <name>
  inputs: {}
  outputs: {}
  ```
  Use `gopkg.in/yaml.v3` to generate this YAML.
- [DONE] Create the corresponding entrypoint file based on language:

  **Rust**:
  - [DONE] Create `src/<name>.rs` with a stub module containing a public function.
  - [DONE] Print a reminder to register the module in `src/lib.rs` or `src/main.rs`.

  **Go**:
  - [DONE] Create a new file at `<name>.go` (or `cmd/<name>.go` if the project uses a cmd layout) with a stub subcommand function.

  **Python**:
  - [DONE] Create `src/<package_name>/<name>.py` with a handler function template and a `run(handler)` call.

  **TypeScript**:
  - [DONE] Create `src/<name>.ts` with a stub handler.

  **R**:
  - [DONE] Create `R/<name>.R` with a stub R script that reads `params.yaml` and writes to `outputs/`.

- [DONE] Print a summary of created files.
- [DONE] Register `addCmd` with `rootCmd` in `init()`.

### Tests (Phase 4)
- [DONE] `cmd/add_test.go`: For each language, set up a temp directory with the language marker file, run `add`, and verify the manifest and entrypoint files are created with correct content.
- [DONE] Test that the block ID follows the `<collection>.<name>` convention.
- [DONE] Test that running `add` outside a collection (no language marker) produces an error.

---

## Phase 5: `spade check` Command

Validates a pipeline file or block collection.

- [DONE] Create `cmd/check.go` defining a `checkCmd` cobra command with `Use: "check [pipeline.yaml]"`, `Short: "Validate a pipeline or block collection"`.
- [DONE] Implement two modes based on arguments:

  **Pipeline validation mode** (`spade check pipeline.yaml`):
  - [DONE] Load the pipeline file using `core.LoadPipeline(path)`.
  - [DONE] Open the block registry at `~/.spade/registry.db` using `core.OpenRegistry`.
  - [DONE] For each block in the pipeline, look up the block manifest from the registry using `registry.LookupBlock(block.Name, "")`. Build a `map[string]core.BlockManifest` for all referenced block types.
  - [DONE] Call `core.ValidatePipeline(pipeline, manifests)` to run the full validation (unique IDs, reference validity, acyclic graph, type compatibility, named output references, required args, map/reduce rules).
  - [DONE] Print all validation errors. If no errors, print a success message.
  - [DONE] Return a non-zero exit code if any errors were found.

  **Collection validation mode** (`spade check` in a collection directory):
  - [DONE] Detect the language using `core.DetectLanguage(".")`.
  - [DONE] Discover all block manifests using `core.DiscoverBlocks(".")`.
  - [DONE] For each manifest, load it with `core.LoadBlockManifest(path)` and check:
    1. [ ] All required fields are present (`id`, `version`, `inputs`, `outputs`).
    2. [ ] Input/output types are valid (one of: `file`, `directory`, `collection`, `string`, `number`, `boolean` for inputs; `file`, `directory`, `collection`, `json`, `expansion` for outputs).
    3. [ ] Block IDs follow the `<collection>.<block>` convention (contains a `.`).
    4. [ ] Entrypoints resolve to existing files or subcommands (check that the source file exists based on language).
    5. [ ] Map blocks have an `expansion` output type.
    6. [ ] Reduce blocks have a `collection` input type.
  - [DONE] Print all validation errors. If no errors, print a success message with the count of validated blocks.
  - [DONE] Return a non-zero exit code if any errors were found.

- [DONE] Register `checkCmd` with `rootCmd` in `init()`.

### Tests (Phase 5)
- [DONE] `cmd/check_test.go`: Test pipeline validation mode with a valid pipeline YAML file and a registry containing the referenced blocks. Verify no errors are returned.
- [DONE] Test pipeline validation with an invalid pipeline (duplicate IDs, cycle, missing references, type mismatch) and verify the correct errors are reported.
- [DONE] Test collection validation with a valid collection directory. Verify success.
- [DONE] Test collection validation with invalid manifests (missing fields, invalid types, bad ID format, missing entrypoint).

---

## Phase 6: `spade install <git-url>` Command

Installs a block collection from a git repository.

- [DONE] Create `cmd/install.go` defining an `installCmd` cobra command with `Use: "install <git-url>"`, `Short: "Install a block collection from a git repository"`, `Args: cobra.ExactArgs(1)`.
- [DONE] Implement the installation process:

  **Step 1: Clone the repository**
  - [DONE] Clone the git URL to a temporary directory using `exec.Command("git", "clone", "--depth=1", url, tmpDir)`.
  - [DONE] Handle clone failures with a clear error message.

  **Step 2: Detect language**
  - [DONE] Call `core.DetectLanguage(tmpDir)` to determine the collection language.

  **Step 3: Discover blocks**
  - [DONE] Call `core.DiscoverBlocks(tmpDir)` to find all block manifests.
  - [DONE] Load each manifest with `core.LoadBlockManifest(path)`.

  **Step 4: Extract version and collection name**
  - [DONE] Read the version from the language's own manifest file:
    - Rust: parse `Cargo.toml` for `[package] version`
    - Go: parse `go.mod` for module name (use directory name as fallback)
    - Python: parse `pyproject.toml` for `[project] version`
    - TypeScript: parse `package.json` for `version`
    - R: default to `"0.1.0"` or read from a `DESCRIPTION` file if present
  - [DONE] Derive the collection name from the language manifest (package name field) or fall back to the repository directory name.

  **Step 5: Run language-specific build**
  - [DONE] **Rust**: Run `cargo build --release` in the cloned directory. The binary is at `target/release/<name>`.
  - [DONE] **Go**: Run `go build -o <name>` in the cloned directory.
  - [DONE] **Python**: Run `uv sync` then `uv tool install .` in the cloned directory.
  - [DONE] **TypeScript**: Run `bun build` in the cloned directory to produce a bundled executable.
  - [DONE] **R**: Run `Rscript setup.R` if present, otherwise install renv dependencies.
  - [DONE] Handle build failures with clear error output (capture and display stderr).

  **Step 6: Install to `~/.spade/blocks/`**
  - [DONE] Create `~/.spade/blocks/<collection>/<version>/` directory.
  - [DONE] Copy the build artifacts (binary for compiled languages, package for interpreted) to the install directory.
  - [DONE] Copy block manifests (`blocks/*.yaml`) to the install directory preserving the `blocks/` subdirectory.

  **Step 7: Register in the block registry**
  - [DONE] Open the registry at `~/.spade/registry.db`.
  - [DONE] For each block manifest, compute the content hash using `core.ComputeContentHash` and register the block using `registry.RegisterBlock` with all fields populated (collection name, version, block name, block ID, language, entrypoint, installed path, content hash, kind, network).
  - [DONE] Close the registry.

  **Step 8: Clean up**
  - [DONE] Remove the temporary clone directory.
  - [DONE] Print a summary of installed blocks.

- [DONE] Register `installCmd` with `rootCmd` in `init()`.

### Tests (Phase 6)
- [DONE] `cmd/install_test.go`: Test installation with a mock git repository (create a temp directory with a valid collection structure, use a `file://` URL for git clone).
- [DONE] Test language detection and version extraction for each language.
- [DONE] Test that blocks are registered in the registry after installation.
- [DONE] Test that installing the same collection twice (same version) updates rather than duplicates.
- [DONE] Test that installing a different version of the same collection installs side-by-side.
- [DONE] Test error handling: invalid git URL, build failure, missing manifests.

---

## Phase 7: `spade run <pipeline.yaml>` Command

Runs a pipeline locally using the single-instance scheduler from the core library.

- [DONE] Rewrite `cmd/run.go` to implement the full pipeline execution.
- [DONE] Define the command with `Use: "run <pipeline.yaml>"`, `Short: "Run a pipeline locally"`, `Args: cobra.ExactArgs(1)`.

### Phase 7.1: Pipeline Loading & Validation
- [DONE] Load the pipeline using `core.LoadPipeline(pipelinePath)`.
- [DONE] Open the block registry at `~/.spade/registry.db`.
- [DONE] For each block in the pipeline, look up the block manifest and registry entry using `registry.LookupBlock(block.Name, "")`. Store both a `map[string]core.BlockManifest` and a `map[string]core.BlockRegistryEntry`.
- [DONE] Validate the pipeline using `core.ValidatePipeline(pipeline, manifests)`. If validation fails, print all errors and exit with code 1.

### Phase 7.2: Working Directory Setup
- [DONE] Create a pipeline working directory at `~/.spade/pipelines/<pipeline_id>/`.
- [DONE] Set up the cache directory at `~/.spade/cache/`.

### Phase 7.3: Scheduler Initialization
- [DONE] Create a `core.SinglePipelineScheduler` using `core.NewSchedulerForPipeline(pipeline)`.
- [DONE] Set the scheduler's `Manifests` field to the loaded manifest map.
- [DONE] Call `scheduler.IdentifyMapContexts()` to detect map/reduce contexts.

### Phase 7.4: Execution Loop
- [DONE] Implement the main execution loop that runs until the pipeline is complete or an error occurs:
  1. [ ] Call `scheduler.Next()` to get the next executable block invocation.
  2. [ ] If `done == true`, the pipeline is complete. Print a summary and exit.
  3. [ ] If no block is ready but pipeline is not done, this indicates a waiting state (should not happen with a single-threaded executor; log a warning and break).
  4. [ ] Look up the block's manifest and registry entry by `invocation.BlockId`.
  5. [ ] **Cache check**: Compute input hashes for the block (from completed blocks' output hashes). Call `core.ComputeCacheKey(manifest.ID, manifest.Version, inputHashes, invocation.Arguments)`. Call `core.CacheLookup(cacheKey, cacheDir)`. If cached:
     - [DONE] Restore cached outputs to the block's working directory using `core.CacheRestore`.
     - [DONE] Create a successful `BlockInvocationResult` and call `scheduler.Update(result)`.
     - [DONE] Print a cache hit message and skip to the next iteration.
  6. [ ] **Input resolution**: Build a `map[uuid.UUID]core.BlockManifest` for the block's dependencies (from completed blocks). Call `core.ResolveInputs(pipelineBlock, depManifests, currentManifest)`.
  7. [ ] **Set up input symlinks**: Call `core.SetupInputSymlinks(workDir, resolvedInputs, pipelineDir)`.
  8. [ ] **For mapped invocations (MapIndex != nil)**: Also call `core.SetupBroadcastInputs` for non-mapped dependencies.
  9. [ ] **Execute the block**: Call `core.Execute(invocation, pipelineDir, manifest, registryEntry, registry)`.
  10. [ ] **Cache store**: If execution was successful, compute the cache key and call `core.CacheStore(cacheKey, outputsDir, cacheDir)`.
  11. [ ] **Update the scheduler**: Call `scheduler.Update(result)`.
  12. [ ] If the result status is `ExecutionStatusError`, print the error details and exit with code 1.

### Phase 7.5: BubbleTea Progress UI
- [DONE] Create a BubbleTea model (`cmd/run_ui.go`) that displays:
  - [DONE] Pipeline name and ID.
  - [DONE] A list of all blocks with their current status (waiting, running, complete, error) using colored indicators.
  - [DONE] The currently executing block name.
  - [DONE] A progress bar or counter showing `completed/total` blocks.
  - [DONE] Elapsed time.
- [DONE] The model receives updates from the execution loop via a channel.
- [DONE] On completion, display a summary: total blocks executed, cached blocks, elapsed time, any errors.
- [DONE] Add a `--no-ui` flag to disable the BubbleTea interface and use simple line-by-line output instead (useful for CI/scripts).

### Phase 7.6: Cleanup
- [DONE] Close the block registry.
- [DONE] Optionally clean up the pipeline working directory (add a `--keep-work-dir` flag to preserve it for debugging).

### Tests (Phase 7)
- [DONE] `cmd/run_test.go`: Test a simple linear pipeline (3 blocks, A -> B -> C) with mock blocks. Verify each block is executed in order and the scheduler state is correct at each step.
- [DONE] Test a pipeline with parallel blocks (diamond DAG: A -> B, A -> C, B -> D, C -> D). Verify all blocks complete.
- [DONE] Test cache hit: execute a pipeline, verify outputs are cached, re-run and verify blocks are skipped from cache.
- [DONE] Test error handling: a block that exits with non-zero code halts the pipeline.
- [DONE] Test `--no-ui` flag produces line-by-line output.

---

## Phase 8: `spade upload` Command

Validates and packages the current collection for upload.

- [DONE] Create `cmd/upload.go` defining an `uploadCmd` cobra command with `Use: "upload"`, `Short: "Upload a block collection for security screening and cloud use"`.
- [DONE] In the `Run` function:
  1. [ ] Run the equivalent of `spade check` (collection validation mode) programmatically. If validation fails, print errors and exit with code 1.
  2. [ ] Detect language using `core.DetectLanguage(".")`.
  3. [ ] Package the collection into a `.tar.gz` archive containing:
     - All source files
     - `blocks/*.yaml` manifests
     - Language manifest file (Cargo.toml, pyproject.toml, etc.)
  4. [ ] Print the path to the generated archive and a message indicating it is ready for upload.
  5. [ ] **Placeholder**: Print a message that the actual upload endpoint is not yet configured. The server-side upload API will be integrated when the PocketBase server is available.
- [DONE] Register `uploadCmd` with `rootCmd` in `init()`.

### Tests (Phase 8)
- [DONE] `cmd/upload_test.go`: Test that upload runs validation first and fails if the collection is invalid.
- [DONE] Test that a valid collection produces a `.tar.gz` archive with the expected contents.

---

## Phase 9: Shared Utilities

- [DONE] Create `cmd/paths.go` with helper functions for resolving standard paths:
  - `SpadeDir() string` — returns `~/.spade/` (or `$SPADE_DIR` if set, for testability).
  - `BlocksDir() string` — returns `~/.spade/blocks/`.
  - `CacheDir() string` — returns `~/.spade/cache/`.
  - `PipelinesDir() string` — returns `~/.spade/pipelines/`.
  - `RegistryPath() string` — returns `~/.spade/registry.db`.
- [DONE] Create `cmd/language.go` with helper functions for extracting collection metadata from language manifests:
  - `ReadCollectionName(repoRoot string, lang core.CollectionLanguage) (string, error)` — reads the collection name from the language manifest (e.g. `name` in `Cargo.toml`, `pyproject.toml`, `package.json`, or directory name for R/Go).
  - `ReadCollectionVersion(repoRoot string, lang core.CollectionLanguage) (string, error)` — reads the version from the language manifest.
- [DONE] Create `cmd/validation.go` with shared validation logic used by both `check` and `upload`:
  - `ValidateCollection(dir string) []error` — runs all collection-level validation checks.

### Tests (Phase 9)
- [DONE] `cmd/paths_test.go`: Test that `SpadeDir()` respects the `$SPADE_DIR` override.
- [DONE] `cmd/language_test.go`: Test `ReadCollectionName` and `ReadCollectionVersion` for each language with mock manifest files.
- [DONE] `cmd/validation_test.go`: Test `ValidateCollection` with valid and invalid collection directories.

---

## Phase 10: Integration Tests

- [DONE] `cmd/integration_test.go`: End-to-end test that exercises the full workflow:
  1. Run `spade setup` to initialize the directory structure.
  2. Run `spade init --language python` to scaffold a Python collection.
  3. Run `spade add myblock` to add a block.
  4. Manually create a simple block implementation (a Python script that copies input to output).
  5. Install the collection from a local `file://` git URL using `spade install`.
  6. Create a minimal pipeline YAML referencing the installed block.
  7. Run `spade check pipeline.yaml` and verify it passes.
  8. Run `spade run pipeline.yaml --no-ui` and verify successful execution.
- [DONE] Test the same workflow for Go and R collections to verify cross-language support.
- [DONE] Test that running `spade run` twice on the same pipeline uses cached results for the second run.

---

## Summary of Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `go.mod` | Modify | Add core library dependency, BubbleTea, uuid, yaml |
| `main.go` | Keep | No changes needed |
| `cmd/root.go` | Modify | Update descriptions, remove toggle flag |
| `cmd/run.go` | Rewrite | Full pipeline execution with scheduler |
| `cmd/run_ui.go` | Create | BubbleTea progress model for pipeline execution |
| `cmd/setup.go` | Create | Setup command |
| `cmd/init.go` | Create | Init command with language scaffolding |
| `cmd/add.go` | Create | Add block command |
| `cmd/check.go` | Create | Check/validation command |
| `cmd/install.go` | Create | Install command |
| `cmd/upload.go` | Create | Upload command |
| `cmd/paths.go` | Create | Shared path helper functions |
| `cmd/language.go` | Create | Language manifest parsing helpers |
| `cmd/validation.go` | Create | Shared collection validation logic |
| `cmd/setup_test.go` | Create | Setup command tests |
| `cmd/init_test.go` | Create | Init command tests |
| `cmd/add_test.go` | Create | Add command tests |
| `cmd/check_test.go` | Create | Check command tests |
| `cmd/install_test.go` | Create | Install command tests |
| `cmd/run_test.go` | Create | Run command tests |
| `cmd/upload_test.go` | Create | Upload command tests |
| `cmd/paths_test.go` | Create | Path helper tests |
| `cmd/language_test.go` | Create | Language helper tests |
| `cmd/validation_test.go` | Create | Validation helper tests |
| `cmd/integration_test.go` | Create | End-to-end integration tests |
