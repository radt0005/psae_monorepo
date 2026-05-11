# CLI Specification

This CLI is designed to enable local development of blocks within the system.  This means that it should have the ability to create and run blocks and pipelines, and should be able to publish them using the system.

## Commands

We are creating a Command Line Interface that uses the following interface:

Usage: `spade [OPTIONS] COMMAND [ARGS]`

Commands
- `run`  Runs the specified pipeline
- `check` Validates the specified pipeline file and all block manifests in a collection
- `install` Installs a block collection from a git repository, a local directory, or the cloud registry
- `publish` Submits a block collection to the cloud registry for screening, build, and distribution
- `init` Creates the boilerplate for a new block collection in the current directory
- `add`  Adds a new block to the current collection
- `setup` Set up the Spade system on the local machine
- `login` Authenticate to the cloud registry


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

- `spade check pipeline.yaml` -- validates a pipeline file (see `pipeline.md` section 8 for validation rules).  If the pipeline contains short codes (see `pipeline.md` section 6), the sibling lockfile (`<pipeline-stem>.lock.yaml`) is generated or updated as a side effect: any short codes not yet bound are assigned a fresh UUIDv7.  Deleting the lockfile is the supported way to regenerate all bindings.
- `spade check` (in a collection directory) -- validates all `blocks/*.yaml` manifests:
  1. All required fields are present
  2. Input/output types are valid
  3. Block IDs follow the `<collection>.<block>` convention
  4. Entrypoints resolve to existing files or subcommands
  5. Map blocks output the `expansion` type, reduce blocks accept `collection` inputs


## `spade install <source>`

Installs a block collection from a git repository, a local directory, or the cloud registry.  `<source>` may be:

- A **registry reference** (e.g. `gdal@1.0.0` or `gdal@latest`) — fetches a prebuilt, signed artifact from the cloud registry.  No local build is performed; this path requires no language toolchains beyond the runtime.  See `registry.md`.
- A **git URL** (`http://`, `https://`, `git://`, `ssh://`, `file://`, or `git@host:path`) — the repository is shallow-cloned into a temp directory and built locally.
- A **local path** (including `.`, `./sub`, and absolute paths) — the directory is used in place and built locally. It does not need to be a git repository.

### Registry-fetch mode

For a registry reference, the process is:

1. Resolve the reference to a concrete version using the registry.
2. Determine the platform/architecture for the local machine.
3. Download the artifact tarball and signature for `<collection>/<version>/<platform>/<arch>`.
4. Verify the signature against the registry's trusted public keys.
5. Verify the artifact's content hash matches the registry metadata.
6. Unpack into `~/.spade/blocks/<collection>/<version>/`.
7. Update the local block index.

The CLI uses the developer's `spade login` session if available, otherwise the registry's public read endpoints.  Workers use a service token instead of a developer session (see `registry.md`).

### Build-from-source mode

For a git URL or local path, the process is:

1. Acquire the source: clone the repository (git URL) or use the local directory as-is.
2. Detect the language from the source root (`Cargo.toml`, `pyproject.toml`, `go.mod`, `package.json`, or R).
3. Discover all blocks by scanning `blocks/*.yaml`.
4. Run the language-specific install:
   - **Rust**: `cargo build --release` (produces a single binary with subcommands)
   - **Go**: `go build` (produces a single binary with subcommands)
   - **Python**: `uv sync` and `uv tool install .`
   - **TypeScript**: `bun build` (bundles into a single executable)
   - **R**: `Rscript setup.R` if present, otherwise install `renv` dependencies
5. Install to `~/.spade/blocks/<collection>/<version>/`.

The version is read from the language's own manifest (e.g. `Cargo.toml`, `pyproject.toml`).  Multiple versions of the same collection can be installed side by side.  Local-path installs build in place, so native toolchains reuse their incremental caches.

Locally-built collections are **not** signed.  The local block index records them as locally built, distinct from registry-fetched artifacts.  This is the developer-facing path; production workers always use registry-fetch.


## `spade run <pipeline.yaml>`

Runs the specified pipeline locally using the single-instance scheduler.  The CLI connects to the Go core package for scheduling and uses the local machine as the sole worker.

Before scheduling, the CLI resolves any short codes in the pipeline against the sibling lockfile, creating or updating it as needed (see `pipeline.md` sections 6.3 and 6.4).  The scheduler and worker see only the resolved (UUID-form) pipeline.


## `spade publish`

Submits the current block collection to the cloud registry for screening, build, and distribution.  Replaces the earlier `spade upload` command.

`spade publish` does **not** upload an artifact.  It submits a `(repo_url, commit_sha, collection_name, version)` reference to the registry, which then clones the repository at the specified SHA, screens the source, builds the artifact in the bundler image, signs it, and stores it.  The build-after-screen ordering is the foundation of the registry's trust chain (see `registry.md`).

The process:

1. Run `spade check` against the working tree.
2. Verify the working tree is clean (no uncommitted changes) and that the current `HEAD` is reachable on the configured remote (no unpushed commits).
3. Resolve the collection name and version from the language's own manifest.
4. Submit `(repo_url, commit_sha, collection_name, version)` to the registry using the developer's `spade login` session.
5. Print the registry URL where the developer can track screening, build, and signing progress.

Because the registry only sees what is reachable on the remote, `spade publish` will refuse to submit if the working tree is dirty or the current commit has not been pushed.  This avoids the "screened source A, deployed source B" trust gap.


## `spade login`

Authenticates the CLI to the cloud registry.  The CLI initiates an OAuth-style flow against the system's identity provider (Better Auth -- see `web_ui.md` and `registry.md`) and stores the resulting session credentials locally for use by `spade publish` and other authenticated commands.

The session is per-user, stored under `~/.spade/auth/`.  `spade logout` clears the stored credentials.

Workers do not use `spade login`; they authenticate to the registry with a rotated service token provisioned at worker setup.


## `spade setup`

Sets up the Spade system on the local machine, including creating the `~/.spade/` directory structure and any required configuration.


## Technology

This CLI should use the following technologies:
- Go language
- Cobra and Viper for the CLI
- BubbleTea for terminal interfaces

The system should connect the Go core package to handle the scheduling, datatypes, etc.  It needs only to run one pipeline at a time.
