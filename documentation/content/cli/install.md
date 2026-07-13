+++
title = "spade install"
description = "Install a block collection from the cloud registry, a Git repository, or a local directory."
weight = 5
+++

The `spade install` command installs a block collection so its blocks can be used in pipelines. It supports two distinct modes depending on what `<source>` looks like:

- **Registry-fetch mode** — `<source>` is a registry reference like `gdal@1.0.0` or `gdal@latest`. Spade downloads a prebuilt, signed artifact from the cloud registry. No local build, no language toolchain required.
- **Build-from-source mode** — `<source>` is a git URL or a local path. Spade acquires the source and builds it locally using the appropriate language toolchain.

Both modes end the same way: blocks are unpacked into `~/.spade/blocks/<collection>/<version>/` and registered in the local block index (`~/.spade/registry.db`). What differs is where the artifact comes from and whether it's signed.

## Usage

```bash
# Registry-fetch mode: a registry reference
spade install gdal@1.0.0
spade install gdal@latest

# Build-from-source mode: a git URL
spade install https://github.com/spade-dev/gdal-blocks.git
spade install git@github.com:spade-dev/gdal-blocks.git

# Build-from-source mode: a local path
spade install .
spade install ./my-collection
spade install /home/user/projects/my-collection
```

`<source>` is interpreted as follows:

| Form | Mode | Example |
|------|------|---------|
| `<collection>@<version>` or `<collection>@latest` | Registry-fetch | `gdal@1.0.0` |
| `http://`, `https://`, `git://`, `ssh://`, `file://`, or `git@host:path` | Build-from-source | `https://github.com/spade-dev/core-blocks.git` |
| Local path (`.`, `./sub`, or absolute) | Build-from-source | `.` |

A local path does not need to be a git repository — the directory is used as-is.

## Registry-fetch mode

For a registry reference, no source is cloned and nothing is compiled. Spade downloads an artifact the registry already built, screened, and signed.

### 1. Resolve the version

`gdal@1.0.0` resolves directly; `gdal@latest` asks the registry for the newest `available` version.

```
Resolved gdal@latest -> 1.3.0
```

### 2. Determine platform and architecture

Spade detects the local machine's platform and architecture (for example, `linux`/`amd64`) to select the matching artifact.

### 3. Download the artifact and signature

The CLI downloads the tarball and its detached signature for `<collection>/<version>/<platform>/<arch>`:

```
<collection>/<version>/<platform>/<arch>.tar.gz
<collection>/<version>/<platform>/<arch>.tar.gz.sig
```

### 4. Verify the signature

The signature is checked against the registry's list of trusted public keys. If verification fails, the artifact is rejected and nothing is installed.

### 5. Verify the content hash

The downloaded tarball's content hash is compared against the value recorded in the registry's metadata for that artifact.

### 6. Unpack

The verified artifact is unpacked into `~/.spade/blocks/<collection>/<version>/`.

### 7. Update the local block index

Each block manifest from the artifact is registered in `~/.spade/registry.db`, marked as a **registry-fetched, signed** install.

```
Resolved gdal@latest -> 1.3.0
Downloading gdal/1.3.0/linux/amd64.tar.gz...
Signature verified.
Content hash verified.
Installed 6 block(s) to /home/user/.spade/blocks/gdal/1.3.0
```

The CLI uses your `spade login` session if one is available, falling back to the registry's public read endpoints for unauthenticated fetches of public collections. Workers use a service token instead of a developer session — see the registry's authentication model in the [Block Collections](/concepts/collections/) overview.

Only collection versions in the `available` state can be fetched this way. Versions that have been `yanked` or `recalled` are refused; see [Block Collections](/concepts/collections/#publishing-and-the-registry) for what those states mean.

## Build-from-source mode

For a git URL or a local path, Spade builds the collection itself using the appropriate language toolchain. This is the developer-facing path — the one you use while iterating on a collection, before (or between) publishing it.

### 1. Acquire the source

- **Git URL**: shallow-cloned (`--depth=1`) into a temporary directory.
- **Local path**: used in place. Not required to be a git repository.

```
Cloning https://github.com/spade-dev/gdal-blocks.git...
```

### 2. Detect the language

Spade examines the source root for language marker files, checked in this order:

| Marker file | Language |
|-------------|----------|
| `Cargo.toml` | Rust |
| `go.mod` | Go |
| `pyproject.toml` | Python |
| `package.json` | TypeScript |
| (none of the above) | R (default) |

```
Detected language: rust
```

### 3. Discover blocks

All `blocks/*.yaml` files are loaded and parsed as block manifests. If none are found, the install fails.

### 4. Read collection metadata

The collection name and version are read from the language-specific manifest:

| Language | Name source | Version source |
|----------|-------------|-----------------|
| Rust | `Cargo.toml` `name` field | `Cargo.toml` `version` field |
| Go | Last path segment of `go.mod` module | Defaults to `0.1.0` |
| Python | `pyproject.toml` `name` field | `pyproject.toml` `version` field |
| TypeScript | `package.json` `name` field | `package.json` `version` field |
| R | Directory name | Defaults to `0.1.0` |

Multiple versions of the same collection can be installed side by side.

### 5. Build

A language-specific build command is run against the source:

| Language | Build command | Notes |
|----------|----------------|-------|
| Rust | `cargo build --release` | Produces a single binary with subcommands |
| Go | `go build` | Produces a single binary with subcommands |
| Python | `uv sync` then `uv tool install .` | Syncs dependencies, then installs the package as a uv tool |
| TypeScript | `bun build` | Bundles into a single executable |
| R | `Rscript setup.R` if present, otherwise install `renv` dependencies | No separate build step beyond dependency resolution |

Local-path installs build in place, so native toolchains (Cargo, Go, etc.) can reuse their incremental build caches across repeated `spade install .` runs.

### 6. Install

The built artifacts and block manifests are installed to `~/.spade/blocks/<collection>/<version>/`.

```
Detected language: rust
Collection: gdal-blocks v1.2.0
Building...
Installed 6 block(s) to /home/user/.spade/blocks/gdal-blocks/1.2.0
```

### 7. Update the local block index

Each block manifest is registered in `~/.spade/registry.db`, marked as **locally built** rather than registry-fetched.

## Signed vs. locally built

This distinction is tracked, not cosmetic:

| | Registry-fetch | Build-from-source |
|---|---|---|
| Artifact origin | Registry, built after screening | Built on your machine |
| Signed | Yes (ed25519, verified on install) | **No** |
| Local block index marks it as | Registry-fetched, signed | Locally built |
| Toolchain required | No | Yes, for the collection's language |
| Used by production workers | Always | Never |

Locally-built collections are **not** signed, and the local block index records them as such. This is intentional: signing only happens after the registry's screening step, so a locally built artifact has no screening signal behind it. Production workers always fetch signed artifacts from the registry; build-from-source is for local development and testing before you publish.

## Verifying the installation

After installing, you can verify that blocks are registered by referencing them in a pipeline and running [`spade check`](/cli/check/):

```bash
spade check my-pipeline.yaml
```

If the pipeline references a block type that was just installed and `spade check` reports it as valid, the installation succeeded.

You can also run `spade setup --rebuild-index` to verify that the filesystem and registry are consistent.

## Examples

Registry-fetch:

```bash
spade install gdal@1.0.0
```

```
Resolved gdal@1.0.0
Downloading gdal/1.0.0/linux/amd64.tar.gz...
Signature verified.
Content hash verified.
Installed 6 block(s) to /home/user/.spade/blocks/gdal/1.0.0
```

Build-from-source, from a git URL:

```bash
spade install https://github.com/spade-dev/gdal-blocks.git
```

```
Cloning https://github.com/spade-dev/gdal-blocks.git...
Detected language: rust
Collection: gdal-blocks v1.2.0
Building...
Installed 6 block(s) to /home/user/.spade/blocks/gdal-blocks/1.2.0
```

Build-from-source, from the current directory (typical during development):

```bash
spade install .
```

```
Detected language: python
Collection: my-collection v0.1.0
Building...
Installed 3 block(s) to /home/user/.spade/blocks/my-collection/0.1.0
```

## See also

- [`spade publish`](/cli/publish/) for submitting a collection to the registry so others can `spade install` the signed artifact
- [`spade login`](/cli/login/) for authenticating registry-fetch requests
- [`spade setup`](/cli/setup/) for initializing the local environment before installing
- [`spade check`](/cli/check/) for verifying the installed blocks work in a pipeline
- [`spade init`](/cli/init/) for creating a new collection to develop locally
