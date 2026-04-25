# Web UI: Gap Analysis

This document inventories the gaps between the current `web_ui/` Nuxt prototype and the specifications in `../spec/`, with a primary focus on `web_ui.md`. It also references `pipeline.md`, `blocks.md`, `scheduler.md`, and `worker.md` because the UI directly produces and consumes the artifacts those documents define.

The gaps are grouped by the spec's UI feature areas, then by supporting concerns (data model, validation, infrastructure).

---

## 1. Pipeline YAML Schema Mismatch (pipeline.md)

The pipeline data model in the prototype is several revisions behind the current `pipeline.md` spec. This is the highest-priority gap because every other UI feature flows through this model.

| Spec (`pipeline.md` §2–§5) | Prototype |
| --- | --- |
| Block field is **`inputs`** (plural array) | `utils/types.ts` `Block.input` (singular) |
| Pipeline has a single **`blocks`** array | `Pipeline` has both `blocks` and `data` arrays (legacy) |
| `inputs` may be **bare ID** *or* **`{block, output}`** explicit reference | Only bare IDs are written/parsed (`utils/pipeline.ts`, `composables/useFlow.ts`) |
| Pipeline `id` is UUIDv7 generated at submission | `PipelineBuilder` mints `v7()` at constructor time |
| `version`, `name`, `description` are top-level fields | Present, but `description` is never editable in the UI |
| Block `id` is UUIDv7 generated **on the client at creation time** so reruns hit the cache | `addNodeFromBlock` re-uses the block's existing id, but `addNode` mints a new UUID and the node-id-vs-block-id wiring in `useFlow` is inconsistent (mixes numeric `nextNodeId` with UUIDs) |

Specific code symptoms:

- `composables/useFlow.ts:117` mints UUIDs for `addNode` but `addNodeFromBlock` (line 170) uses `${nextNodeId.value}` for the returned id while pushing `block.id` into the node — two different ID spaces are leaking into the same store.
- `utils/pipeline.ts:26-40` writes `input: []` (singular). It also splits nodes between `blocks` and `data` based on `node.data.is_data`, a flag that does not exist in the spec.
- `composables/useFlow.ts:88` reads `block.input` (singular) when importing YAML, so it cannot round-trip a spec-conformant pipeline that uses `inputs:`.
- The example pipeline hard-coded in `server/api/pipeline/run.post.ts:19-70` still uses `input:` and a `data: []` section.

---

## 2. Input/Output Wiring (web_ui.md §"Input/Output Wiring") — **NOT IMPLEMENTED**

This is called out as a first-class requirement in `web_ui.md` and is currently absent from the prototype.

The spec requires:

1. When two blocks are connected, the UI must **resolve which outputs feed which inputs by type**.
2. **Unambiguous matches** are made silently.
3. **Ambiguous matches** must prompt the user to choose, producing explicit `{block, output}` entries in the YAML.
4. The UI must **show the `description` field** from each input/output during the prompt.
5. **Pipelines with unresolved ambiguous connections must not be savable or submittable.**

Prototype state:

- `components/flow/Base.vue:31` calls `flow.connect(source, target)` which only appends an `Edge`. No resolution, no prompt, no `output` field is stored on the edge.
- `utils/pipeline.ts:42` resolves edges to `block.input.push(input_id)` only — explicit references are never produced.
- The block schemas the editor consumes (`/api/schema/[id]`) come from PocketBase as a *JSON Schema for parameters* (see `blocks/random-forest-fit.json`). They contain **no `inputs`/`outputs` declarations**, so the UI has no way to perform type-based resolution even if the wiring code existed.

---

## 3. Block Manifest Model (blocks.md §3)

The block-manifest model the UI relies on does not match the spec.

Spec block manifest (YAML):

```yaml
id: gdal.rasterize
version: 1.0.0
kind: standard | map | reduce
network: false
description: ...
inputs:
  vectors:
    type: file
    format: GeoJSON
    description: ...
outputs:
  raster:
    type: file
    format: GeoTIFF
    description: ...
```

Prototype block manifest (JSON, stored in PocketBase `schema` field, e.g. `blocks/random-forest-fit.json`):

```json
{
  "name": "handler",
  "description": "...",
  "parameters": { "type": "object", "properties": { ... }, "required": [ ... ] }
}
```

What is missing or wrong:

- No `id`, `version`, `kind`, `network`, `inputs`, `outputs` declarations.
- No notion of `type` / `format` / `item_type` per input/output.
- No `kind: map` / `kind: reduce` distinction. The `is_data` flag in PocketBase is a prototype-only concept that the spec replaces with declared inputs/outputs.
- The block list endpoint (`server/api/blocks/list.ts`) only surfaces `name`, `label`, `type: "block"` — discarding everything the UI needs to render block metadata.

---

## 4. Map / Reduce Support (scheduler.md §"Map and Reduce", pipeline.md §8)

There is **no UI representation of map / reduce semantics** today.

Required by the spec but absent:

- No visual distinction for `kind: map` or `kind: reduce` nodes.
- No handling of the `expansion` output type.
- No handling of `collection` input types.
- No surfacing of map context propagation (downstream-of-map blocks run N times).
- No validation that a map context is closed by a reduce before another map begins.

---

## 5. Pipeline Storage, Browsing, Sharing (web_ui.md §"Pipelines")

> "There should be a way to store and share pipelines for future use…store the pipeline YAML files…it should be possible to browse past pipelines and re-load one in the editor."

Prototype state:

- Pipelines are not stored as a first-class entity. `Export.vue:38-46` writes a YAML blob into the `runs` PocketBase collection only as part of submitting a run. There is **no `pipelines` collection** and no way to save a pipeline without running it.
- `pages/index.vue:13` has an "Edit an Existing Pipeline" link that points to `/editor` but does not load anything.
- No "browse pipelines" page exists.
- No sharing UI (per-pipeline access control, share links).
- The "Import" dialog (`components/Import.vue`) only accepts a pasted YAML string — there is no library to pick from.

---

## 6. Custom Data Upload & Sharing (web_ui.md §"User Interface for Pipeline Generation")

> "There should be a way to upload (potentially large) files and use the custom data in pipelines. There should also be a way to share this data as well without re-uploading."

Prototype state:

- A `user-file-upload` entry exists in the **hard-coded list** in `components/flow/DataEditor.vue:18` but no upload component, no storage, no listing, no sharing exist.
- `utils/files.ts` is empty.
- `server/api/data/list.ts` returns `{ items: [] }` — a stub.
- Geospatial files (which the spec calls out as large) get no special handling; nothing chunks or resumes uploads.

---

## 7. Result Browsing & Download (web_ui.md §"Browsing Results")

> "Browse results, view results that are small enough, and download results through the UI. This should include large files as well."

Prototype state (`pages/results/[id].vue`, `components/results/*`):

- Viewers exist for CSV, JSON, GeoJSON, image, text, stdout/stderr — good coverage for small files.
- **Missing**:
  - GeoTIFF / raster preview (the system is "first-class geospatial").
  - Tabular formats beyond CSV (Parquet, Arrow).
  - Range/streamed download for large files; the current `downloadFile` flow loads the entire URL into an `<a download>` click.
  - "Download all" zips nothing — it just fires per-file downloads in a loop (`pages/results/[id].vue:99-105`).
  - Result sharing (web_ui.md item 7).
  - File-name presentation is mangled by `cleanupFileName` (`pages/results/[id].vue:127-130`) which assumes `name_suffix.ext` and silently corrupts other formats.

---

## 8. Block Browsing (web_ui.md §item 6)

> "Browse blocks"

Prototype state:

- The only place the user encounters blocks is the `<USelectMenu>` inside the block editor slide-over (`components/flow/BlockEditor.vue:190`).
- No dedicated `/blocks` browsing page, no descriptions, no input/output preview, no example usages.
- No filtering by collection, kind, or capability.

---

## 9. Pipeline Validation (pipeline.md §7, web_ui.md §"Input/Output Wiring")

`spade check` validations have no equivalent in the UI. The spec is explicit that the web UI should catch ambiguity at authoring time. None of the following are checked client-side:

1. Unique block ids within the pipeline.
2. All referenced ids exist in the block list.
3. All `name` values refer to known installed blocks.
4. DAG is acyclic.
5. Type compatibility on connections.
6. Named output references exist on the dependency block's manifest.
7. All required `args` are present.

The submit button (`Export.vue:32-52`) writes whatever YAML the user produced, regardless of validity.

---

## 10. Authentication & Access Control (web_ui.md §"…limit access to authorized users.")

- PocketBase auth works (`composables/usePB.ts`, `middleware/auth.ts`, `components/Login.vue`).
- **`components/SignUp.vue:14-17` is broken** — it calls `authWithPassword` (login) where it should call `users.create` then sign in. New users cannot register through the UI.
- The PocketBase URL is **hard-coded** in `composables/usePB.ts:4` to `https://acg-floating-204-197-5-169.acg.maine.edu/`. Should come from `runtimeConfig` / env.
- No per-pipeline / per-result ACLs needed for the sharing features in §5 and §7.
- No password reset, no email confirmation flow (`pages/confirm.vue` exists but is empty).

---

## 11. Look & Feel (web_ui.md §"Look and Feel")

> "The user interface should have the same look and feel as the website and documentation website. This should include the same color palette, icons, and overall feel."

Prototype state:

- Default NuxtUI theme. No brand color palette, typography, or icon set carried over from the marketing site or docs site.
- Homepage (`pages/index.vue`) is three plain `<UCard>` links and "Welcome to the Homepage!".
- `app.vue` is just `<NuxtLayout><NuxtPage/></NuxtLayout>` (97-byte file).
- `assets/`, `public/`, no shared theme tokens.

---

## 12. Infrastructure & Code-Health Gaps

These are not user-visible but block the spec features above.

- Hard-coded absolute paths to `/home/krbundy/.psae/runs/` in `server/api/pipeline/save.post.ts:21`, `server/api/results/[id]/index.get.ts:7`, and `[name].get.ts:16`. Won't run on any other machine or in Docker.
- `server/api/pipeline/run.post.ts:19-70` ignores the user's pipeline and **submits a hard-coded example pipeline** to RabbitMQ. The actual submission path used by `Export.vue` writes to PocketBase `runs` directly and bypasses this endpoint, so this file is dead but misleading.
- RabbitMQ URL hard-coded to `amqp://localhost` (`server/api/pipeline/run.post.ts:8`).
- `pages/run.vue` and `pages/run/[id].vue` are near-identical stubs that poll a non-existent endpoint.
- `composables/useFlow.ts` mixes `nextNodeId` (numeric) with `v7()` UUIDs and never increments `nextNodeId` consistently — a refactor hazard once we start trusting block ids for caching.
- `block.schema.json` and `block-index.json` at the repo root are stale local fixtures (DuckDB block) that no code path reads — they should be deleted or moved to `dev-fixtures/`.
- The two `run` pages, the empty `utils/files.ts`, the unused `SchemaForm2.vue` (different forms library), and `components/flow/Options.vue` / `DataEditor.vue` (superseded by `BlockEditor.vue` with `:data="true"`) are accumulating dead code.
- `nuxt.config.ts` has `ssr: false` — fine for the editor, but worth confirming for SEO of public landing/browse pages.

---

## 13. Spec Items Without Any Implementation Hook

Cross-referencing `web_ui.md`'s opening checklist against the prototype:

| web_ui.md feature | Status |
| --- | --- |
| 1. Create and run pipelines via flowchart | Partial — flowchart works, run path uses wrong YAML schema |
| 2. View results | Partial — viewers exist, geo/tabular gaps remain |
| 3. Download result files | Partial — single-file works, no large-file / bulk |
| 4. Re-use pipelines | **Missing** — no pipeline library |
| 5. Share pipelines | **Missing** |
| 6. Browse blocks | **Missing** — only a dropdown |
| 7. Share results | **Missing** |
| 8. Upload custom data | **Missing** — stubbed only |
| Auth & authorization | Partial — login works, signup broken, no ACLs |
| Input/Output wiring resolution | **Missing entirely** |
| Look & feel matching marketing / docs site | **Missing** |

---

## Summary

The prototype is roughly the **flowchart-editor + run-monitor** slice of the spec, built against an older pipeline schema and an older block-manifest schema. To reach the current `web_ui.md` we need to (a) catch the data model up to `pipeline.md` and `blocks.md`, (b) build the connection-resolution UX that `web_ui.md` describes in detail, and (c) add the four "missing entirely" feature areas: pipeline library, block browser, custom-data upload, and result sharing.

See `IMPLEMENTATION_PLAN.md` for the step-by-step plan.
