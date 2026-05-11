# The `spade` CLI

The `spade` CLI is the local development tool for the Spade system. It scaffolds collections, adds blocks, validates manifests and pipelines, installs collections from git, and runs pipelines locally using the single-instance scheduler.

The binary lives at `cli/spade` in this monorepo (built from `cli/`). Build with `go build -o spade .` from `cli/`. Requires Go 1.25+ and a C compiler (for SQLite via CGo).

---

## Command summary

```
spade setup                  Set up the local environment (~/.spade/, registry)
spade init -l <language>     Scaffold a new block collection in the current directory
spade add <name>             Add a block to the current collection
spade check [pipeline.yaml]  Validate a pipeline file or the current collection
spade install <git-url|path> Install a block collection from a git repo or local directory
spade run <pipeline.yaml>    Run a pipeline locally
spade upload                 Validate and package the current collection for upload
```

---

## `spade setup`

```
spade setup
spade setup --rebuild-index
```

Creates the local Spade directory tree and initializes the SQLite registry. Run this once on a new machine.

It creates:
- `~/.spade/blocks/` — installed block collections
- `~/.spade/cache/` — execution cache
- `~/.spade/pipelines/` — pipeline working directories
- `~/.spade/registry.db` — SQLite block registry

`--rebuild-index` rescans `~/.spade/blocks/`, re-reads every `blocks/*.yaml`, recomputes content hashes, and rebuilds the registry from scratch. Use it if the registry gets out of sync with the filesystem (or as a recovery step after a crash). The filesystem under `~/.spade/blocks/` is always the source of truth; the registry is an index.

The `SPADE_DIR` environment variable overrides the default `~/.spade/` path — useful for tests and isolated dev environments.

---

## `spade init`

```
spade init --language <rust|go|python|typescript|r>
spade init -l python
```

Scaffolds a new block collection in the current directory. The `--language` flag is required; there is no interactive prompt.

What gets created depends on the language:

| Language     | Files                                                            |
| ------------ | ---------------------------------------------------------------- |
| `rust`       | `Cargo.toml`, `src/lib.rs`, `blocks/`                            |
| `go`         | `go.mod`, `main.go`, `blocks/`                                    |
| `python`     | `pyproject.toml`, `src/<package>/__init__.py`, `blocks/`         |
| `typescript` | `package.json`, `src/`, `blocks/`                                |
| `r`          | `renv.lock`, `R/`, `blocks/`                                     |

The `blocks/` directory is created empty — add blocks with `spade add`. The collection name comes from the directory name; the language manifest's `name` field uses that name.

After scaffolding, the collection is ready to receive blocks. There's no separate "build" step in Spade — compilation uses the language's native tools (`cargo build`, `go build`, `bun build`, `uv sync`, …) and is invoked by `spade install` when the collection is installed.

---

## `spade add <name>`

```
spade add reproject
spade add classify
```

Adds a new block to the current collection. Must be run from a collection directory (one with the language manifest at the root). The CLI auto-detects the language from the root file.

What it creates:

1. **`blocks/<name>.yaml`** — a manifest template with `id` set to `<collection>.<name>`, `version: 0.1.0`, `kind: standard`, and empty `inputs`/`outputs` maps. **You must fill in the inputs and outputs yourself.**
2. **An entry point file** in the language's standard location:
    - **Rust:** `src/<name>.rs` with a stub `pub fn run()`. You also need to register the module in `src/lib.rs` and `src/main.rs`.
    - **Go:** `<name>.go` at the package root with a stub function.
    - **Python:** `src/<package>/<name>.py` with a `handler(params)` skeleton and an `if __name__ == "__main__":` guard.
    - **TypeScript:** `src/<name>.ts` with an exported `handler` skeleton.
    - **R:** `R/<name>.R` with a script reading `params.yaml`.

The generated handler is a placeholder that doesn't yet use the runtime library. After running `spade add`, replace the stub with a real handler (see `references/blocks.md` for the language-specific templates) and fill in the manifest.

---

## `spade check`

```
spade check                  # validate the current collection
spade check pipeline.yaml    # validate a pipeline file
```

Without arguments, validates every `blocks/*.yaml` in the current directory:

- Required fields are present
- Input/output types are valid
- Block IDs follow `<collection>.<block>`
- Entrypoints resolve to existing files or subcommands
- `kind: map` blocks output `type: expansion`
- `kind: reduce` blocks accept a `type: collection` input

With a pipeline file, validates that pipeline against the registry:

- All block invocation IDs are unique (UUIDs or short codes)
- All referenced invocation IDs resolve to a block in the pipeline
- All `name` values resolve to installed blocks in the registry
- The dependency graph is acyclic
- Input/output types are compatible across edges
- Explicit `output` references match actual outputs in the dependency block's manifest
- All required `args` are present
- Map/reduce constraints are satisfied (no nested maps, expansion/collection shapes correct)
- Short code grammar; lockfile validity if present

If the pipeline uses short codes (e.g. `"@reproject"`), `spade check` also creates or updates the sibling lockfile `<pipeline-stem>.lock.yaml`, minting fresh UUIDv7s for any short codes not yet bound. Delete the lockfile to regenerate all bindings from scratch.

Run `spade check` after editing manifests or pipelines and before `spade run`. It catches the common authoring mistakes early.

Exit code is non-zero on validation failure; errors are printed to stderr.

---

## `spade install <git-url | path>`

```
spade install https://github.com/example/gdal-blocks.git
spade install file:///path/to/local/repo
spade install .                  # install from the current directory
spade install ./my-collection    # install from a local path
```

Installs a block collection from a git repository **or** a local directory. The source argument chooses the mode:

- If it looks like a URL (`http://`, `https://`, `git://`, `ssh://`, `file://`) or an SCP-style git ref (`git@host:path`), it is treated as a **git URL**.
- Otherwise it is treated as a **local path**. The path must exist and be a directory; it does not need to be a git repository.

The process:

1. **Source acquisition.** Git URLs are shallow-cloned into a temp directory. Local paths are used in place — no clone happens and the build runs in the user's working tree, so `cargo`, `go`, `uv`, and `bun` can reuse their incremental caches.
2. Detect the language from the source root.
3. Discover blocks by scanning `blocks/*.yaml`.
4. Read the collection name and version from the language manifest.
5. Run the language-specific build:
   - **Rust:** `cargo build --release` (single binary with subcommands)
   - **Go:** `go build -o <name>` (single binary with subcommands)
   - **Python:** `uv sync`, then `uv tool install .`
   - **TypeScript:** `bun build .` (bundled executable)
   - **R:** `Rscript setup.R` if present, else no build step
6. Copy the built artifacts and `blocks/*.yaml` files to `~/.spade/blocks/<collection>/<version>/`.
7. Compute a content hash for the install directory and register every block in the SQLite registry.

Multiple versions of the same collection can be installed side by side. Install is **trust-on-install**: the build commands run as the current user, unsandboxed — same trust model as `cargo install` or `pip install`. The cloud upload path adds security screening.

---

## `spade run <pipeline.yaml>`

```
spade run pipeline.yaml
spade run pipeline.yaml --no-ui
spade run pipeline.yaml --keep-work-dir
```

Runs a pipeline locally using the single-instance scheduler. The local machine is the sole worker. The CLI:

1. Loads and validates the pipeline.
2. **Resolves short codes** against the sibling `<pipeline-stem>.lock.yaml`, creating or updating it as needed. The scheduler and worker only see the resolved (UUID-form) pipeline.
3. Looks up every referenced block in the registry and loads its manifest.
4. Validates the pipeline against the manifests.
5. Creates a working directory under `~/.spade/pipelines/<pipeline-id>/`.
6. Walks the dependency graph, executing each block in turn (mapped invocations are scheduled per-item, broadcast inputs are symlinked into every mapped invocation).
7. Restores from cache when `(block id, version, input hashes, args)` matches a previous run.
8. Stores cache entries on successful execution.
9. Halts the pipeline on the first non-zero exit, preserving logs.

Flags:
- `--no-ui` — disable the BubbleTea TUI and print one line per block. Use this in non-interactive contexts (CI, logs).
- `--keep-work-dir` — preserve `~/.spade/pipelines/<pipeline-id>/` after execution. Useful for inspecting intermediate inputs/outputs and logs.

The working directory is removed by default at the end of a run — pass `--keep-work-dir` if you need to debug.

---

## `spade upload`

```
spade upload
```

Validates the current collection (runs `spade check` first), then packages it into a `.tar.gz` archive for upload to the cloud system. Cloud uploads go through security screening before they become installable on production workers.

This command does not (currently) push the archive itself — it just produces the archive. Use the cloud system's upload UI/API for the actual transfer.

---

## Configuration

`spade` reads configuration from `~/.spade.yaml` (or a path passed via `--config`). Viper-style environment variables also work. The most useful override is:

- **`SPADE_DIR`** — overrides the default `~/.spade/` directory. Useful for tests, ephemeral environments, or running multiple Spade installs side by side.

---

## End-to-end developer workflow

A typical day building a new collection:

```bash
# One-time
spade setup

# Create a collection
mkdir my-blocks && cd my-blocks
spade init --language python

# Add blocks
spade add reproject
spade add classify
# ... edit blocks/reproject.yaml and src/my_blocks/reproject.py
# ... edit blocks/classify.yaml and src/my_blocks/classify.py

# Validate
spade check

# Install it locally so the registry can find the blocks
spade install .

# Write a pipeline
$EDITOR pipeline.yaml

# Validate and run it
spade check pipeline.yaml
spade run pipeline.yaml --keep-work-dir
```

For local iteration on a collection you're actively editing, you'll need to re-run `spade install` after every change so the registry picks up the new build artifacts.
