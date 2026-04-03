+++
title = "spade upload"
description = "Package a collection for cloud deployment."
weight = 7
+++

The `spade upload` command validates the current block collection and packages it as a compressed archive for cloud deployment. It is run from the root of a block collection directory.

## Usage

```bash
spade upload
```

No arguments are required. The command operates on the collection in the current working directory.

## What it does

### 1. Validate the collection

The command runs the same validation checks as [`spade check`](/cli/check/) (without arguments). If any errors are found, the command prints them to stderr and exits with status 1 without creating an archive. This ensures that only valid collections can be packaged.

### 2. Detect language and read metadata

The collection language is auto-detected from marker files in the project root, and the collection name and version are read from the language-specific manifest:

| Language | Marker file | Name source | Version source |
|----------|-------------|-------------|----------------|
| Rust | `Cargo.toml` | `name` field | `version` field |
| Go | `go.mod` | Module path (last segment) | Defaults to `0.1.0` |
| Python | `pyproject.toml` | `name` field | `version` field |
| TypeScript | `package.json` | `name` field | `version` field |
| R | (none) | Directory name | Defaults to `0.1.0` |

### 3. Create the archive

A `.tar.gz` archive is created in the current directory, named `<collection>-<version>.tar.gz`. The archive contains:

**Always included:**
- `blocks/*.yaml` -- All block manifest files

**Language manifest:**

| Language | Included file |
|----------|--------------|
| Rust | `Cargo.toml` |
| Go | `go.mod` |
| Python | `pyproject.toml` |
| TypeScript | `package.json` |
| R | `renv.lock` |

**Source files:**

| Language | Included sources |
|----------|-----------------|
| Rust | `src/` directory (recursive) |
| Go | All `*.go` files in the project root |
| Python | `src/` directory (recursive) |
| TypeScript | `src/` directory (recursive) |
| R | `R/` directory (recursive) |

Build artifacts, test files, and version control metadata are not included in the archive.

## Example

```bash
cd my-collection
spade upload
```

Output on success:

```
Collection packaged: my-collection-0.1.0.tar.gz
Note: Upload endpoint is not yet configured. The server-side upload API will be integrated when the PocketBase server is available.
```

Output on validation failure:

```
Collection validation failed with 1 error(s):
  - block normalize: missing required field 'version'
```

## Cloud endpoint

The upload destination is not yet configured. When the cloud infrastructure becomes available, `spade upload` will submit the archive to a server endpoint for security screening and deployment. Currently, the command only produces the local `.tar.gz` file.

## See also

- [`spade check`](/cli/check/) for running validation independently
- [`spade init`](/cli/init/) for creating a new collection
- [`spade install`](/cli/install/) for installing collections locally
