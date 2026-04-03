+++
title = "spade init"
description = "Scaffold a new block collection."
weight = 2
+++

The `spade init` command scaffolds a new block collection project in the current directory. A collection is a set of related blocks that share a language and are versioned together.

## Usage

```bash
spade init --language <language>
spade init -l <language>
```

The `--language` (or `-l`) flag is required. Supported values are:

| Value | Language marker file |
|-------|---------------------|
| `rust` | `Cargo.toml` |
| `go` | `go.mod` |
| `python` | `pyproject.toml` |
| `typescript` | `package.json` |
| `r` | `renv.lock` |

The collection name is derived from the current directory name.

## Scaffolded structures

Each language produces a different project layout. In all cases, a `blocks/` directory is created for block manifests.

### Rust

```bash
mkdir my-collection && cd my-collection
spade init -l rust
```

```
my-collection/
  Cargo.toml        # [package] with name and version
  src/
    lib.rs           # Collection library root
  blocks/            # Block manifest YAML files
```

The generated `Cargo.toml`:

```toml
[package]
name = "my-collection"
version = "0.1.0"
edition = "2021"
```

### Go

```bash
mkdir my-collection && cd my-collection
spade init -l go
```

```
my-collection/
  go.mod             # Module declaration
  main.go            # Collection entry point
  blocks/            # Block manifest YAML files
```

The generated `go.mod` uses the Go version of your current toolchain. The `main.go` file contains a stub `main()` function.

### Python

```bash
mkdir my-collection && cd my-collection
spade init -l python
```

```
my-collection/
  pyproject.toml           # Project metadata
  src/
    my_collection/         # Package directory (hyphens converted to underscores)
      __init__.py
  blocks/                  # Block manifest YAML files
```

The generated `pyproject.toml`:

```toml
[project]
name = "my-collection"
version = "0.1.0"
requires-python = ">=3.10"
```

### TypeScript

```bash
mkdir my-collection && cd my-collection
spade init -l typescript
```

```
my-collection/
  package.json       # Package metadata with main entry
  src/               # Source directory for block handlers
  blocks/            # Block manifest YAML files
```

The generated `package.json`:

```json
{
  "name": "my-collection",
  "version": "0.1.0",
  "main": "src/index.ts"
}
```

### R

```bash
mkdir my-collection && cd my-collection
spade init -l r
```

```
my-collection/
  renv.lock          # renv lockfile for dependency management
  R/                 # R scripts for block handlers
  blocks/            # Block manifest YAML files
```

## Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--language` | `-l` | (required) | Language for the collection: `rust`, `go`, `python`, `typescript`, or `r` |

## Workflow after scaffolding

After running `spade init`, the typical next steps are:

1. **Add a block.** Use [`spade add <name>`](/cli/add/) to create a block manifest and source file.
2. **Implement the handler.** Edit the generated source file to perform actual processing.
3. **Validate.** Run [`spade check`](/cli/check/) to verify manifests and entrypoints.
4. **Install locally.** Run [`spade install file://.`](/cli/install/) to build and register the collection.
5. **Use in a pipeline.** Reference your blocks by their `<collection>.<block>` name in a pipeline YAML file.

## Language detection

Spade detects a collection's language by checking for marker files in the project root, in this order:

1. `Cargo.toml` -- Rust
2. `go.mod` -- Go
3. `pyproject.toml` -- Python
4. `package.json` -- TypeScript
5. If none of these are found, the collection defaults to R

This detection order is used by `spade add`, `spade check`, `spade install`, and `spade upload` whenever they need to determine the language of an existing collection.

## See also

- [`spade add`](/cli/add/) for adding blocks to the scaffolded collection
- [Your First Block](/getting-started/first-block/) for a step-by-step tutorial
- [Library documentation](/libraries/) for language-specific details
