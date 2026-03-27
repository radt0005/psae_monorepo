# CLI Specification

This CLI is designed to enable local development of blocks within the system.  This means that it should have the ability to create and run blocks and pipelines, and should be able to publish them using the system.

## Commands

We are creating a Command Line Interface that uses the following interface:

Usage: `spade [OPTIONS] COMMAND [ARGS]`

Commands
- `run`  Runs the specified pipeline
- `check` Validates the specified pipeline file and all block manifests in a collection
- `install` Installs a block collection from a git repository
- `upload` Uploads a block collection for security screening and use on the cloud
- `init` Creates the boilerplate for a new block collection in the current directory
- `add`  Adds a new block to the current collection
- `setup` Set up the Spade system on the local machine


## `spade init`

Creates a new block collection in the current directory.  The user selects a language, and the CLI scaffolds the appropriate project structure:

- **Rust**: `Cargo.toml`, `src/lib.rs`, `blocks/`
- **Go**: `go.mod`, `main.go`, `blocks/`
- **Python**: `pyproject.toml`, `src/<package>/`, `blocks/`
- **TypeScript**: `package.json`, `src/`, `blocks/`
- **R**: `renv.lock`, `R/`, `blocks/`

The `blocks/` directory is created empty.  Blocks are added with `spade add`.


## `spade add <name>`

Adds a new block to the current collection.  This creates:

1. A block manifest at `blocks/<name>.yaml` with a template (id, version, kind, inputs, outputs)
2. A corresponding entry point in the source tree:
   - **Rust**: a new module in `src/<name>.rs` and a subcommand registration
   - **Go**: a new subcommand file
   - **Python**: `src/<package>/<name>.py` with a handler function and `run()` call
   - **TypeScript**: `src/<name>.ts`
   - **R**: `R/<name>.R`


## `spade check`

Validates a pipeline file or block collection:

- `spade check pipeline.yaml` -- validates a pipeline file (see `pipeline.md` section 7 for validation rules)
- `spade check` (in a collection directory) -- validates all `blocks/*.yaml` manifests:
  1. All required fields are present
  2. Input/output types are valid
  3. Block IDs follow the `<collection>.<block>` convention
  4. Entrypoints resolve to existing files or subcommands
  5. Map blocks output the `expansion` type, reduce blocks accept `collection` inputs


## `spade install <git-url>`

Installs a block collection from a git repository.  The process:

1. Clone the repository using `git`
2. Detect the language from the repository root (`Cargo.toml`, `pyproject.toml`, `go.mod`, `package.json`, or R)
3. Discover all blocks by scanning `blocks/*.yaml`
4. Run the language-specific install:
   - **Rust**: `cargo build --release` (produces a single binary with subcommands)
   - **Go**: `go build` (produces a single binary with subcommands)
   - **Python**: `uv sync` and `uv tool install .`
   - **TypeScript**: `bun build` (bundles into a single executable)
   - **R**: `Rscript setup.R` if present, otherwise install `renv` dependencies
5. Install to `~/.spade/blocks/<collection>/<version>/`

The version is read from the language's own manifest (e.g. `Cargo.toml`, `pyproject.toml`).  Multiple versions of the same collection can be installed side by side.


## `spade run <pipeline.yaml>`

Runs the specified pipeline locally using the single-instance scheduler.  The CLI connects to the Go core package for scheduling and uses the local machine as the sole worker.


## `spade upload`

Validates and packages the current collection for upload to the cloud system.  This runs `spade check` first, then packages the collection for security screening and deployment.


## `spade setup`

Sets up the Spade system on the local machine, including creating the `~/.spade/` directory structure and any required configuration.


## Technology

This CLI should use the following technologies:
- Go language
- Cobra and Viper for the CLI
- BubbleTea for terminal interfaces

The system should connect the Go core package to handle the scheduling, datatypes, etc.  It needs only to run one pipeline at a time.
