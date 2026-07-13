+++
title = "Installation"
description = "Install the Spade CLI and set up your local environment."
weight = 1
+++

## System Requirements

Spade runs on **Linux** and **macOS**. You need:

- A terminal with a POSIX-compatible shell (bash, zsh)
- Git (for building block collections from source with `spade install`; not needed for registry-fetch installs like `spade install gdal@latest`)

For block development, you also need the toolchain for your chosen language — see the [library documentation](/libraries/) for specifics.

## Install the Spade CLI

The Spade CLI is a single Go binary. Install it with:

```bash
go install github.com/spade-dev/spade/cli@latest
```

This places the `spade` binary in your `$GOPATH/bin` directory. Make sure that directory is in your `PATH`.

{% note() %}
You need **Go 1.25 or later** to build the CLI from source. If you don't have Go installed, download it from [go.dev](https://go.dev/dl/).
{% end %}

## Initialize Spade

After installing, run the setup command to create the local Spade directory:

```bash
spade setup
```

This creates `~/.spade/` with the following structure:

```
~/.spade/
  auth/        # Session credentials from `spade login`
  blocks/      # Installed block collections
  cache/       # Execution cache for block outputs
  pipelines/   # Working directories for pipeline runs
  registry.db  # SQLite registry of installed blocks
```

## Verify the installation

Check that everything is working:

```bash
spade --help
```

You should see output listing all available commands:

```
Spade - A data processing system for massive data

Usage:
  spade [command]

Available Commands:
  run         Run a pipeline locally
  check       Validate blocks or pipelines
  install     Install a block collection from the cloud registry, a git repository, or a local directory
  publish     Submit a block collection to the cloud registry for screening, build, and distribution
  init        Create a new block collection
  add         Add a new block to the current collection
  setup       Set up the Spade environment
  login       Authenticate to the cloud registry
  secret      Manage local and cloud secrets for pipeline runs
```

## Custom install location

By default, Spade stores everything in `~/.spade/`. To use a different location, set the `SPADE_DIR` environment variable:

```bash
export SPADE_DIR=/path/to/custom/spade
spade setup
```

## Language toolchains

To develop blocks, you need the appropriate language toolchain installed:

| Language | Requirement | Install guide |
|----------|------------|---------------|
| Python | Python 3.12+, `uv` | [Python library](/libraries/python/) |
| R | R 4.0+, `renv` | [R library](/libraries/r/) |
| TypeScript | Bun runtime, TypeScript 5.0+ | [TypeScript library](/libraries/typescript/) |
| Go | Go 1.25+ | [Go library](/libraries/go/) |
| Rust | Rust stable, Cargo | [Rust library](/libraries/rust/) |

## Next steps

Now that Spade is installed, continue to [Your First Pipeline](/getting-started/first-pipeline/) to run a simple workflow.
