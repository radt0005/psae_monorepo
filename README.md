# Spade

Spade is a plugin-based, geospatial-first data processing system. Workflows are
authored as **pipelines** — declarative DAGs in YAML — and executed as a
collection of **blocks**: isolated, language-agnostic units of computation that
read inputs from a working directory and write outputs to a designated output
directory. Pipelines can fan out across collections of items using `map` and
`reduce` blocks, and the scheduler runs independent steps in parallel across
workers that share a filesystem.

This monorepo contains the full system: the design specs, the CLI, the core
scheduler/worker library, runtime libraries for block authors in five
languages, the first-party block collections, the web client, and the
documentation site.

## Repository layout

| Directory            | Contents                                                                                          |
| -------------------- | ------------------------------------------------------------------------------------------------- |
| `spec/`              | Design specifications. The source of truth for the system's architecture and contracts.           |
| `cli/`               | The `spade` CLI (Go, Cobra/Viper, BubbleTea). Local development tooling and pipeline runner.      |
| `core/`              | Go core library: types, scheduler, registry, executor, validation. Shared between CLI and worker. |
| `runner/`            | Single-instance pipeline runner used by `spade run`.                                              |
| `server/`            | Server layer (PocketBase): authentication and job submission for the web client.                  |
| `web_ui/`            | Web client — flowchart-based pipeline editor, results viewer, block browser.                      |
| `website/`           | Public-facing project website.                                                                    |
| `documentation/`     | User-facing documentation site (Zola).                                                            |
| `libs/`              | Runtime libraries for block authors, one per supported language.                                  |
| `libs/python/`       | Python runtime library.                                                                           |
| `libs/R/`            | R runtime library.                                                                                |
| `libs/rust/`         | Rust runtime library.                                                                             |
| `libs/go/`           | Go runtime library.                                                                               |
| `libs/typescript/`   | TypeScript (Bun) runtime library.                                                                 |
| `blocks/`            | First-party block collections.                                                                    |
| `blocks/base/`       | Common data operations (Rust).                                                                    |
| `blocks/gdal/`       | GDAL wrappers for geospatial operations.                                                          |
| `blocks/data/`       | Data-provider blocks built on OpenDAL, plus a catalog of known sources.                           |
| `blocks/ml/`         | Machine-learning blocks.                                                                          |
| `blocks/sae/`        | Small-area-estimation blocks.                                                                     |
| `prompt_templates/`  | Prompt templates used by the system.                                                              |
| `skills/`            | Claude Code skills for working with the system.                                                   |
| `skills/spade/`      | The `spade` skill — distilled references for authoring blocks and pipelines.                      |
| `test/`              | Cross-cutting integration tests.                                                                  |

## Architecture

The system has six components:

1. **Scheduler** — decides which block runs when, on which worker. Maintains
   execution order from the pipeline DAG and handles map/reduce expansion.
2. **Workers** — execute blocks as sandboxed subprocesses. Workers share a
   filesystem, set up each invocation's working directory, symlink dependency
   outputs into the next block's inputs, and report results back to the
   scheduler. Sandboxing uses Ubuntu `isolate`.
3. **Web client** — flowchart-based GUI for authoring, running, and sharing
   pipelines and viewing results.
4. **Server** — PocketBase-backed layer for authentication, pipeline storage,
   and job submission.
5. **CLI (`spade`)** — local development tooling: scaffolds collections,
   validates manifests and pipelines, installs collections from git, and runs
   pipelines locally against the single-instance scheduler.
6. **Blocks** — the plugins that do the actual work, distributed in
   collections.

### Blocks

A block is a reusable, isolated unit of computation that declares its inputs,
outputs, and parameters in a YAML manifest and runs as a standalone process.
Each invocation is given a fresh working directory containing `params.yaml`,
an `inputs/` tree (one subdirectory per declared input), an `outputs/` tree,
and a `logs/` directory. Blocks read by name, not by filename, and write into
the matching output subdirectory. They have no network access unless the
manifest declares `network: true`.

Blocks are distributed in **collections**: one repository per language, with
manifests in `blocks/*.yaml` and handlers in the language's standard source
layout. The collection language is detected from the root manifest
(`Cargo.toml`, `go.mod`, `pyproject.toml`, `package.json`, otherwise R), and
the collection is installed to `~/.spade/blocks/<collection>/<version>/`.

### Pipelines

A pipeline is a YAML document listing block invocations and the dependencies
between them. The scheduler builds a DAG from the `inputs` references — bare
invocation IDs are resolved by type matching against the block manifests, and
explicit `block` + `output` references disambiguate cases where type matching
alone is not enough. Block invocation IDs are UUIDv7 and stable across reruns
of the same pipeline, which lets the cache skip blocks whose inputs and
parameters have not changed.

Parallel "for each" operations are expressed with `kind: map` and
`kind: reduce` blocks. A map block writes an expansion manifest enumerating
items in a collection; the scheduler then creates N invocations of each
downstream block in the map context, broadcasting non-mapped dependencies
to every invocation. A reduce block closes the map context by collecting the
N outputs back into a single invocation.

## Supported languages

Block authors can write handlers in **Python**, **R**, **Rust**, **Go**, or
**TypeScript (Bun)**. Each runtime library exposes typed input/output classes
and a `run()` function that loads parameters, scans the inputs directory, calls
the user's handler, and writes the returned values to the appropriate output
subdirectories.

## Working with this repo via Claude

The `skills/spade/` skill provides a self-contained reference for routine
work — authoring blocks, writing pipelines, and using the CLI. Reach for
`spec/` only when a question goes beyond what the skill covers (scheduler
internals, the worker sandbox model, the block registry, or the web UI's
ambiguity-resolution rules).
