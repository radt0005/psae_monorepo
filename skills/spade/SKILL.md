---
name: spade
description: Develop for the Spade data processing system. Use whenever the user is creating or modifying Spade blocks, writing or validating pipelines, scaffolding block collections, or working with the `spade` CLI. Triggers on phrases like "add a block", "block manifest", "block.yaml", "pipeline.yaml", "spade init", "spade run", "spade check", "scaffold a collection", "map block", "reduce block", "expansion manifest", or any time the user is editing files under `blocks/` of a Spade collection or under `./spec/`, `./libs/`, `./cli/`, or `./blocks/` in this monorepo. Use it even when the user does not say "Spade" by name — if you see a `blocks/<name>.yaml` file, a `pipeline.yaml`, or a handler that imports `spade`, this skill applies.
---

# Spade

Spade is a plugin-based, geospatial-first data processing system. Work happens in two units:

- **Blocks** — isolated units of computation, distributed in **collections** (one per language). Each block has a YAML manifest and a handler in source code.
- **Pipelines** — declarative YAML DAGs that wire block invocations together.

The `spade` CLI scaffolds collections, adds blocks, validates manifests and pipelines, installs collections from git, and runs pipelines locally.

This skill is self-contained: everything needed to create blocks, write pipelines, and use the CLI is here in `references/`. You generally do **not** need to read `./spec/` to do routine work — the references are distilled from the spec and kept in sync. Only fall back to `./spec/` if a user question goes beyond what's in the references (for example, internals of the scheduler, the worker security model, or the registry implementation).

## How to use this skill

Pick the reference file that matches the user's task, **read it before writing code**, and then act:

| Task                                              | Read first                         |
| ------------------------------------------------- | ---------------------------------- |
| User says "create / add a block", or asks about a `block.yaml` / handler / collection | `references/blocks.md`             |
| User wants to write or fix a `pipeline.yaml`, wire blocks together, add map/reduce | `references/pipelines.md`          |
| User asks about a `spade ...` command, scaffolding, installing, running, validating | `references/cli.md`                |
| User asks "what type should I use", or about `format`, `item_type`, raster vs vector | `references/types.md`              |

If the task spans more than one (e.g. "add a new block and use it in a pipeline"), read each relevant reference. The references are short and cross-link.

## Repository orientation

When working inside this monorepo, the relevant directories are:

- `cli/` — the `spade` CLI source (Go, Cobra). Built binary lives at `cli/spade`.
- `core/` — Go core library: types, scheduler, registry, executor, validation.
- `libs/` — runtime libraries for block authors, one per language: `python/`, `R/`, `rust/`, `go/`, `typescript/`.
- `blocks/` — first-party block collections (`base/`, `data/`, `gdal/`, `sae/`). Use these as worked examples when authoring new blocks in the same language.
- `spec/` — design specs. Reach for these only when the references are insufficient.
- `documentation/` — public documentation site (Zola).

When the user is editing inside a block collection (a directory with `Cargo.toml`/`go.mod`/`pyproject.toml`/`package.json`/`renv.lock` plus a `blocks/` subdirectory), treat that directory as the working unit.

## Defaults and conventions to apply automatically

These are not negotiable conventions of the Spade system; apply them without asking unless the user has been explicit otherwise:

1. **Block IDs follow `<collection>.<block>`.** The collection name comes from the language manifest (`name` in `Cargo.toml` / `pyproject.toml` / `package.json`, or the directory name for Go/R).
2. **Every input and output gets a `description`.** The web UI shows these to users wiring pipelines, and `spade check` does not enforce them, but blocks without descriptions are hostile to use.
3. **`network: false` unless the block genuinely needs the internet.** Network access is opt-in for security; only add `network: true` when the block calls an external API or downloads data.
4. **Filenames determine block names.** `blocks/rasterize.yaml` defines the `rasterize` block. The default `entrypoint` is the filename stem; only set `entrypoint` explicitly if the language toolchain needs a different name (e.g. a non-default `uv` script).
5. **Blocks must not assume filenames or directories outside their working dir.** Read inputs from `inputs/<name>/`, write outputs to `outputs/<name>/`, read scalars from `params.yaml`. The runtime libraries handle all of this if you use them.
6. **After creating or editing manifests, suggest `spade check`.** It catches the common mistakes (missing fields, bad references, wrong map/reduce shapes, ambiguous wiring) before the user runs anything.
7. **Pipelines use UUIDv7 for IDs.** When hand-authoring a pipeline, generate UUIDv7s rather than v4. If a UUIDv7 is unavailable, any well-formed UUID will parse, but UUIDv7 is the convention.

## Workflow for the most common requests

**"Add a block called X to this collection"**
1. Detect the language by looking at the root manifest file (`Cargo.toml` → Rust, `go.mod` → Go, `pyproject.toml` → Python, `package.json` → TypeScript, otherwise R).
2. Read `references/blocks.md` for the manifest schema and language-specific handler template.
3. Create `blocks/<name>.yaml` with `id`, `version`, `kind`, `inputs`, `outputs`, and descriptions.
4. Create the handler at the language's standard location (see the language section in `references/blocks.md`).
5. Tell the user to run `spade check` (or run it yourself if appropriate).

**"Write a pipeline that does X"**
1. Read `references/pipelines.md`.
2. Identify the blocks needed and look up their inputs/outputs (from `blocks/*.yaml` in the relevant collection, or from the user's description).
3. Generate UUIDv7s for the pipeline and each block.
4. Wire dependencies — prefer bare references when type matching is unambiguous, explicit `block`+`output` when not.
5. Add `kind: map` / `kind: reduce` blocks if the pipeline fans out over a collection.
6. Suggest `spade check pipeline.yaml`.

**"How do I use the CLI to do X"**
- Read `references/cli.md` and answer from it. Do not guess flags.

## When to read the spec instead

Fall back to `./spec/*.md` only when the user is asking about system internals not covered by the references — for example:

- How the scheduler handles map context propagation across multiple downstream blocks (`spec/scheduler.md`)
- The worker's `isolate`-based sandbox model (`spec/worker.md`) — and remember, the registered preference is `isolate`, not `go-landlock`
- The block registry SQLite schema (`spec/worker.md`)
- How the web UI resolves ambiguous output wiring (`spec/web_ui.md`)

For day-to-day block and pipeline authoring, the references in this skill are the source of truth.
