+++
title = "spade install"
description = "Install a block collection from a Git repository."
weight = 5
+++

The `spade install` command clones a block collection from a Git repository, builds it, copies the artifacts to `~/.spade/blocks/`, and registers each block in the local registry.

## Usage

```bash
spade install <git-url>
```

The argument is any URL that `git clone` accepts. This includes HTTPS URLs, SSH URLs, and `file://` URLs for local directories.

## Step-by-step process

The install command performs the following steps in order:

### 1. Clone the repository

The repository is shallow-cloned (`--depth=1`) into a temporary directory. This avoids downloading the full history.

```
Cloning https://github.com/spade-dev/core-blocks.git...
```

### 2. Detect language

Spade examines the cloned repository root for language marker files, checked in this order:

| Marker file | Language |
|-------------|----------|
| `Cargo.toml` | Rust |
| `go.mod` | Go |
| `pyproject.toml` | Python |
| `package.json` | TypeScript |
| (none of the above) | R (default) |

```
Detected language: python
```

### 3. Discover blocks

All `*.yaml` files in the `blocks/` directory are loaded and parsed as block manifests. If no manifests are found, the install fails.

### 4. Read collection metadata

The collection name and version are read from the language-specific manifest file:

| Language | Name source | Version source |
|----------|-------------|----------------|
| Rust | `Cargo.toml` `name` field | `Cargo.toml` `version` field |
| Go | Last path segment of `go.mod` module | Defaults to `0.1.0` |
| Python | `pyproject.toml` `name` field | `pyproject.toml` `version` field |
| TypeScript | `package.json` `name` field | `package.json` `version` field |
| R | Directory name | Defaults to `0.1.0` |

```
Collection: core-blocks v0.3.0
```

### 5. Build

A language-specific build command is executed in the cloned directory:

| Language | Build command | Notes |
|----------|--------------|-------|
| Rust | `cargo build --release` | Produces a binary at `target/release/<name>` |
| Go | `go build -o <name>` | Produces a binary at `./<name>` |
| Python | `uv sync` then `uv tool install .` | Syncs dependencies, then installs the package as a uv tool |
| TypeScript | `bun build .` | Produces a bundled binary |
| R | `Rscript setup.R` (if `setup.R` exists) | No build step if `setup.R` is absent |

### 6. Copy to install directory

The built artifacts and block manifests are copied to `~/.spade/blocks/<collection>/<version>/`:

| Language | What is copied |
|----------|---------------|
| Rust | `blocks/*.yaml` + compiled binary |
| Go | `blocks/*.yaml` + compiled binary |
| Python | `blocks/*.yaml` + `src/` directory tree |
| TypeScript | `blocks/*.yaml` + bundled binary |
| R | `blocks/*.yaml` + `R/` directory tree |

### 7. Register in the block registry

Each block manifest is registered in `~/.spade/registry.db` with:

- Collection name and version
- Block ID and name
- Language and entrypoint
- Install path
- Content hash (SHA-256 of the installed directory)
- Block kind (`standard`, `map`, or `reduce`)
- Network access flag

```
Installed 4 block(s) to /home/user/.spade/blocks/core-blocks/0.3.0
```

The temporary clone directory is removed after installation completes.

## Installing from local directories

To install a collection from a local directory without pushing to a remote, use a `file://` URL:

```bash
# Install from the current directory
spade install file://.

# Install from an absolute path
spade install file:///home/user/projects/my-collection
```

This is useful during development when iterating on a collection before publishing it.

## Verifying the installation

After installing, you can verify that blocks are registered by referencing them in a pipeline and running [`spade check`](/cli/check/):

```bash
spade check my-pipeline.yaml
```

If the pipeline references a block type that was just installed and `spade check` reports it as valid, the installation succeeded.

You can also run `spade setup --rebuild-index` to verify that the filesystem and registry are consistent.

## Example

```bash
spade install https://github.com/spade-dev/gdal-blocks.git
```

Full output:

```
Cloning https://github.com/spade-dev/gdal-blocks.git...
Detected language: rust
Collection: gdal-blocks v1.2.0
Building...
Installed 6 block(s) to /home/user/.spade/blocks/gdal-blocks/1.2.0
```

## See also

- [`spade setup`](/cli/setup/) for initializing the local environment before installing
- [`spade check`](/cli/check/) for verifying the installed blocks work in a pipeline
- [`spade init`](/cli/init/) for creating a new collection to develop locally
