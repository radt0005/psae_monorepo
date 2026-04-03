+++
title = "Block Collections"
description = "Repositories of related blocks sharing a language and build system."
weight = 2
+++

A **block collection** is a repository that groups related blocks together. All blocks in a collection share the same programming language and build system. Collections are the unit of distribution in Spade: you install, version, and publish collections, not individual blocks.

## What collections are for

Collections serve several purposes:

- **Organization** — Group related blocks by domain (e.g., raster processing, machine learning, data ingestion)
- **Shared code** — Blocks in the same collection can share utility functions, data types, and dependencies
- **Consistent toolchain** — All blocks in a collection use the same language and dependency management
- **Versioning** — The entire collection is versioned together, so all its blocks stay in sync

For example, you might have a `raster` collection containing blocks for reprojection, resampling, and mosaic operations, all written in Python and sharing a common set of GDAL bindings.

## Directory structure

A collection has this general layout:

```
my-collection/
  blocks/
    reproject.yaml      # Block manifest
    resample.yaml       # Block manifest
    mosaic.yaml         # Block manifest
  src/                  # Language-specific source code
    ...
  <project-file>        # Language-specific project file (see below)
```

The `blocks/` directory contains one YAML manifest per block. Each manifest declares the block's interface (inputs, outputs, parameters) as described in [Blocks](/concepts/blocks/). The rest of the repository contains the source code and configuration for the chosen language.

## Language detection

Spade determines the collection's language by looking for a project file in the repository root:

| Project file | Language detected |
|-------------|-------------------|
| `Cargo.toml` | Rust |
| `go.mod` | Go |
| `pyproject.toml` | Python |
| `package.json` | TypeScript |
| *(none of the above)* | R (default) |

When you run `spade init --language <lang>`, Spade scaffolds the appropriate project file and directory structure. If you create a collection manually, just make sure the correct project file is present at the root.

### Language-specific layouts

**Python** collections use `pyproject.toml` and place source code under `src/<package_name>/`:

```
my-collection/
  pyproject.toml
  blocks/
    summarize.yaml
  src/
    my_collection/
      __init__.py
      summarize.py
```

**R** collections are the default when no recognized project file is found. Source code lives under `R/`:

```
my-collection/
  DESCRIPTION
  blocks/
    interpolate.yaml
  R/
    interpolate.R
```

**Go** collections use `go.mod`:

```
my-collection/
  go.mod
  go.sum
  blocks/
    convert.yaml
  cmd/
    convert/
      main.go
```

**Rust** collections use `Cargo.toml`:

```
my-collection/
  Cargo.toml
  blocks/
    detect.yaml
  src/
    bin/
      detect.rs
```

**TypeScript** collections use `package.json`:

```
my-collection/
  package.json
  tsconfig.json
  blocks/
    transform.yaml
  src/
    transform.ts
```

## Block ID convention

Every block has a unique ID in the form `<collection>.<block>`. The collection name comes from the repository or directory name, and the block name comes from the manifest filename (without the `.yaml` extension).

For example, a collection named `raster` containing a manifest at `blocks/reproject.yaml` produces a block with the ID `raster.reproject`.

This naming convention ensures block IDs are globally unique and immediately tell you which collection a block belongs to. When you reference a block in a pipeline, you use this full `<collection>.<block>` ID.

## Versioning

Collections use [semantic versioning](https://semver.org/) (e.g., `1.0.0`, `0.3.2`). The version is specified in each block's manifest file under the `version` field. All blocks in a collection should share the same version number, since they are built and installed together.

When you install a new version of a collection, it is stored alongside previous versions. This means multiple versions of the same collection can coexist, and pipelines can pin to a specific version if needed.

## Installing collections

Collections are installed from Git repositories or local directories using the `spade install` command:

```bash
# From a Git repository
spade install https://github.com/spade-dev/core-blocks.git

# From a local directory
spade install file://.
```

When you install a collection, Spade:

1. Clones or copies the source
2. Detects the language from the project file
3. Builds the collection using the appropriate toolchain (e.g., `pip install` for Python, `cargo build` for Rust)
4. Copies the built artifacts and manifests to `~/.spade/blocks/<collection>/<version>/`

The installed layout looks like:

```
~/.spade/blocks/
  raster/
    0.2.1/
      blocks/
        reproject.yaml
        resample.yaml
        mosaic.yaml
      <built artifacts>
    0.3.0/
      ...
  ml/
    1.0.0/
      blocks/
        classify.yaml
      <built artifacts>
```

## Creating a new collection

To create a new collection:

```bash
mkdir my-blocks && cd my-blocks
spade init --language python
```

This scaffolds the project structure for the chosen language. Then add blocks with:

```bash
spade add my-block-name
```

This creates the manifest file at `blocks/my-block-name.yaml` and a starter source file in the appropriate location for the language.

Validate your collection at any time:

```bash
spade check
```

This verifies that all manifests are well-formed, entrypoints exist, and there are no conflicting block IDs.
