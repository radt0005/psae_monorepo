# Spade CLI

Command-line tool for developing, installing, and running geospatial data processing pipelines in the Spade system.

## Installation

```bash
go build -o spade .
```

Requires Go 1.25+ and a C compiler (for SQLite via CGo).

## Quick Start

```bash
# Set up the local environment
spade setup

# Create a new block collection
mkdir my-blocks && cd my-blocks
spade init --language python

# Add a block to the collection
spade add reproject

# Validate the collection
spade check

# Install a collection from a git repository
spade install https://github.com/example/gdal-blocks.git

# Validate a pipeline
spade check pipeline.yaml

# Run a pipeline locally
spade run pipeline.yaml
```

## Commands

### `spade setup`

Creates the `~/.spade/` directory structure and initializes the block registry.

```bash
spade setup
spade setup --rebuild-index   # rebuild registry from installed blocks
```

Creates:
- `~/.spade/blocks/` -- installed block collections
- `~/.spade/cache/` -- execution cache
- `~/.spade/pipelines/` -- pipeline working directories
- `~/.spade/registry.db` -- block registry (SQLite)

### `spade init`

Scaffolds a new block collection in the current directory.

```bash
spade init --language python
spade init -l rust
```

Supported languages: `rust`, `go`, `python`, `typescript`, `r`

Each language generates the appropriate project structure with a `blocks/` directory for block manifests.

### `spade add <name>`

Adds a new block to the current collection. Creates a block manifest at `blocks/<name>.yaml` and a corresponding entrypoint file in the source tree.

```bash
spade add reproject
spade add classify
```

### `spade check [pipeline.yaml]`

Validates a pipeline file or block collection.

```bash
# Validate a pipeline
spade check pipeline.yaml

# Validate all blocks in the current collection
spade check
```

Pipeline validation checks: unique block IDs, valid references, acyclic dependency graph, type compatibility, required arguments, and map/reduce rules.

Collection validation checks: required manifest fields, valid input/output types, block ID conventions, entrypoint resolution, and map/reduce constraints.

If the pipeline uses short codes (`@<identifier>`) instead of UUIDs for block invocation IDs, `spade check` also creates or updates the sibling lockfile `<pipeline-stem>.lock.yaml`, minting fresh UUIDv7s for any short codes not yet bound. Delete the lockfile to regenerate all bindings from scratch. See `spec/pipeline.md` §6 for the short-code system.

### `spade install <git-url | path>`

Installs a block collection from a git repository or a local directory.

```bash
spade install https://github.com/example/gdal-blocks.git
spade install file:///path/to/local/repo
spade install .                  # install from current directory
spade install ./my-collection    # install from a local path
```

Git URLs are shallow-cloned into a temp directory. Local paths are built in place — no clone is performed and the directory does not need to be a git repository. Either way, the collection is language-detected, built, copied to `~/.spade/blocks/<collection>/<version>/`, and registered in the block registry.

### `spade run <pipeline.yaml>`

Runs a pipeline locally using the single-instance scheduler.

```bash
spade run pipeline.yaml
spade run pipeline.yaml --no-ui          # simple line output
spade run pipeline.yaml --keep-work-dir  # preserve working directory
```

Loads the pipeline, validates it, resolves block manifests from the registry, and executes blocks in dependency order. Supports caching -- repeated runs skip blocks whose inputs haven't changed.

Short-code pipelines are resolved against the sibling `<pipeline-stem>.lock.yaml` lockfile before scheduling, with bindings minted on first run and reused on subsequent runs (preserving cache hits). See `spec/pipeline.md` §6.

### `spade upload`

Validates and packages the current collection into a `.tar.gz` archive for upload to the cloud system.

```bash
spade upload
```

## Configuration

Spade reads configuration from `~/.spade.yaml` (or a path specified with `--config`). Environment variables are also supported via Viper.

The `SPADE_DIR` environment variable overrides the default `~/.spade/` directory, useful for testing or non-standard setups.

## Project Layout

```
cli/
  main.go              # entry point
  go.mod               # module definition (depends on ../core)
  cmd/
    root.go            # root command and config
    setup.go           # spade setup
    init.go            # spade init
    add.go             # spade add
    check.go           # spade check
    install.go         # spade install
    run.go             # spade run
    upload.go          # spade upload
    paths.go           # ~/.spade/ path helpers
    language.go        # language manifest parsing
    validation.go      # shared collection validation
```

## Dependencies

- [Cobra](https://github.com/spf13/cobra) -- CLI framework
- [Viper](https://github.com/spf13/viper) -- configuration management
- [core](../core/) -- Spade core library (scheduling, execution, validation, caching, registry)

## Testing

```bash
go test ./... -v
```
