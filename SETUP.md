# Setup

This guide walks through everything needed to get a working Spade development
environment from scratch on Linux or macOS. By the end you will have:

- All of the language toolchains required by the first-party block collections
- The `spade` CLI built and installed
- The Rust block collections (`base` and `data`) compiled
- All of the first-party block collections installed and ready to use in
  pipelines

The instructions assume you have already cloned this monorepo and are running
commands from its root.

> **Tip:** once the toolchains in section 1 are installed, you can run
> `./setup.sh` from the repo root to automate sections 2–5 (build the CLI,
> run `spade setup`, build the Rust collections, install all collections,
> and validate).

## 1. Language toolchains

Spade is a polyglot system. The CLI is written in Go, two of the first-party
collections are Rust, two are Python, and one is R. You need each toolchain
because `spade install` builds each collection with its own native tools.

- Go: [Download and install - The Go Programming Language](https://go.dev/doc/install)
  
- Rust: [Installation Instructions](https://rust-lang.org/tools/install/)
  
- Python 3.11+: [Download Python | Python.org](https://www.python.org/downloads/)
  
- After installing Python, install the `uv` package manager with `pip install uv`.  Alternatively, you can install from [Astral](https://docs.astral.sh/uv/getting-started/installation/).
  
- Install R:  [Installation Instructions](https://www.r-project.org)
  
- Install the Bun run-time (for front-end system): [Installation Instructions](https://bun.sh)

- Isolate [https://github.com/ioi/isolate](https://github.com/ioi/isolate) on Linux systems.  See the installation instructions in the repository

### 1.7 System libraries (geospatial)

The `blocks/gdal` collection wraps GDAL and needs the system library
available at build and run time.

See the installation instructions for GDAL from their [downloads page](https://gdal.org/en/stable/download.html)

Verify with:

```bash
gdalinfo --version
```

## 2. Build and install the `spade` CLI

From the repo root, install the CLI with `go install`. This compiles the
binary and places it in `$(go env GOPATH)/bin` (typically `~/go/bin`).

```bash
cd cli
go install .
cd ..
```

Confirm it is on your `PATH` and runs:

```bash
spade --help
```

If `spade` is not found, make sure `$(go env GOPATH)/bin` is on your `PATH`
(see the Go installation step above).

Then initialize the local Spade directory structure:

```bash
spade setup
```

This creates `~/.spade/` and the `~/.spade/blocks/` tree where installed
collections will live.

## 3. Compile the Rust block collections

The `base` and `data` collections are Rust. Build each one with `cargo`
before installing — this catches compile errors early and warms the
incremental build cache that `spade install` will reuse.

```bash
cargo build --release --manifest-path blocks/base/Cargo.toml
cargo build --release --manifest-path blocks/data/Cargo.toml
```

If both builds succeed you should have release binaries under
`blocks/base/target/release/base` and `blocks/data/target/release/data`.

## 4. Install all of the first-party block collections

`spade install <path>` detects the language of each collection from its root
manifest, runs the appropriate build, and installs the result to
`~/.spade/blocks/<collection>/<version>/`. Run it for each first-party
collection:

```bash
spade install blocks/base
spade install blocks/data
spade install blocks/gdal
spade install blocks/ml
spade install blocks/sae
```

What each one does under the hood:

- `blocks/base` — `cargo build --release` (Rust)
- `blocks/data` — `cargo build --release` (Rust)
- `blocks/gdal` — `uv sync` and `uv tool install .` (Python; needs system
  GDAL from step 1.7)
- `blocks/ml` — `uv sync` and `uv tool install .` (Python)
- `blocks/sae` — bootstraps `renv` and installs R dependencies (R)

## 5. Verify the installation

List what's installed and run a quick validation pass:

```bash
ls ~/.spade/blocks/
spade check
```

You should see one entry per installed collection, and `spade check` should
report no errors for the manifests that ship in this repo.

You're ready to author blocks and run pipelines. See:

- `skills/spade/SKILL.md` — task-oriented references for authoring blocks and
  pipelines
- `spec/` — full system specifications
- `cli/README.md` — additional CLI documentation
