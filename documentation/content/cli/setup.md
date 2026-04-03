+++
title = "spade setup"
description = "Initialize the local Spade environment."
weight = 1
+++

The `spade setup` command creates the local Spade directory structure and initializes the block registry. Run this once after installing the CLI, or any time you need to recreate the environment.

## Usage

```bash
spade setup
```

## What it creates

The command creates the following directory tree (by default at `~/.spade/`):

```
~/.spade/
  blocks/       # Installed block collections
  cache/        # Content-addressed cache of block outputs
  pipelines/    # Working directories for pipeline runs
  registry.db   # SQLite registry of installed blocks
```

Each subdirectory is created with `0755` permissions. The `registry.db` file is a SQLite database that tracks every installed block, its collection, version, language, entrypoint, and content hash.

## Flags

| Flag | Description |
|------|-------------|
| `--rebuild-index` | Rebuild the block registry by scanning the `blocks/` directory on the filesystem |

### `--rebuild-index`

If the registry database becomes out of sync with the installed block files (for example, after manually copying or removing block directories), the `--rebuild-index` flag re-scans `~/.spade/blocks/` and repopulates `registry.db` from what it finds on disk:

```bash
spade setup --rebuild-index
```

This is a non-destructive operation -- it reads the filesystem and updates the database to match. It does not remove any installed files.

## Custom install location

By default, Spade stores everything under `~/.spade/`. To use a different location, set the `SPADE_DIR` environment variable before running `setup`:

```bash
export SPADE_DIR=/data/spade
spade setup
```

All subsequent Spade commands will use the directory specified by `SPADE_DIR`. If the variable is unset, they fall back to `~/.spade/`.

## When to use it

- **After first install.** The CLI will not function correctly without the directory structure and registry database.
- **After upgrading Spade.** If a new version changes the registry schema, `spade setup` will apply any migrations.
- **After manual filesystem changes.** If you move, copy, or delete block directories by hand, run `spade setup --rebuild-index` to bring the registry back into sync.
- **On a new machine.** If you copy your `~/.spade/blocks/` tree to another machine, run `spade setup --rebuild-index` to initialize the registry from the existing files.

## Example output

```
  Created /home/user/.spade
  Created /home/user/.spade/blocks
  Created /home/user/.spade/cache
  Created /home/user/.spade/pipelines
  Initialized registry at /home/user/.spade/registry.db
Spade setup complete.
```

## See also

- [Installation guide](/getting-started/installation/) for installing the Spade CLI binary
- [`spade install`](/cli/install/) for installing block collections into the environment
