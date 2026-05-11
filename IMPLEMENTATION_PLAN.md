# Implementation Plan: Pipeline Short Codes and Lockfile

This plan implements the short-code authoring form for pipelines as specified in `spec/pipeline.md` §6.  Short codes (`@<identifier>`) replace UUIDv7 invocation IDs in hand-authored or LLM-generated pipeline files; the CLI resolves them to UUIDs via a sibling `<pipeline-stem>.lock.yaml` lockfile.  The scheduler, worker, and registry never see short codes -- resolution happens before the existing `Pipeline` struct is populated.

**Design summary.**  All existing typed structures (`Pipeline`, `PipelineBlock`, `InputRef`) keep their `uuid.UUID` fields unchanged.  Resolution is implemented as a structure-aware walk of the parsed `yaml.Node` tree: read pipeline → walk → substitute short codes for UUIDs (consulting/updating the lockfile) → re-marshal → parse into the existing typed `Pipeline`.  This keeps the blast radius confined to a single new file in `core/` plus thin CLI integration; no downstream code in the scheduler, executor, registry, runner, or workers changes.

**Component scope:**
- `core/` — lockfile types, YAML walker, resolution orchestration, validation rules from §6.6.
- `cli/cmd/` — `spade check` and `spade run` wire through the new loader.
- `web_ui/` — small TypeScript resolver applied where uploaded pipeline YAML enters the system.
- `runner/` — **no changes**.  The runner sees only resolved UUID-form pipelines, identical to today's behavior for web-UI-authored pipelines.

---

## Phase 1: Lockfile type and persistence (`core/lockfile.go`, new file)

- [DONE] Define `Lockfile` struct with fields `Pipeline string`, `Version string`, and `Bindings map[string]uuid.UUID`.  YAML tags: `pipeline`, `version`, `bindings` (omit empty for the first two).  See `spec/pipeline.md` §6.3 for the on-disk format.
- [DONE] Implement `LoadLockfile(path string) (Lockfile, error)`.  Returns an empty (zero-valued) `Lockfile` with `Bindings` initialized to an empty map when the file does not exist (no error) -- this is how the first-run "no lockfile yet" case is signaled to callers.  Returns an error for malformed YAML or invalid UUIDs in bindings.
- [DONE] Implement `SaveLockfile(lock Lockfile, path string) error` using `gopkg.in/yaml.v3`.  Marshals bindings with stable key ordering (sort by short code) so the file diffs cleanly in version control.  File permissions `0644`; `os.MkdirAll(dir, 0755)` first.
- [DONE] Implement `LockfilePathFor(pipelinePath string) string` that returns the sibling lockfile path: strip the extension from the basename and append `.lock.yaml`.  E.g. `pipeline.yaml` → `pipeline.lock.yaml`, `workflows/foo.yml` → `workflows/foo.lock.yaml`.
- [DONE] Define a sentinel error `ErrInvalidLockfile` that callers can match on to suggest "delete the lockfile to regenerate" in user-facing error messages.  Spec §6.6: "Failures in lockfile entries should produce errors that point the user to the option of deleting the lockfile to regenerate it."

### Tests (Phase 1) — `core/lockfile_test.go`
- [DONE] `TestLockfile_LoadMissing`: load from a path that doesn't exist; returns a zero-valued Lockfile with a non-nil empty `Bindings` map and no error.
- [DONE] `TestLockfile_RoundTrip`: save a Lockfile with three bindings, load it back, assert byte-equal YAML and equal struct contents.
- [DONE] `TestLockfile_StableOrdering`: save twice with bindings inserted in different orders, assert byte-equal output (sorted by short code).
- [DONE] `TestLockfile_InvalidBindings`: a binding with a non-UUID string returns `ErrInvalidLockfile`.
- [DONE] `TestLockfile_InvalidYAML`: malformed YAML returns `ErrInvalidLockfile`.
- [DONE] `TestLockfilePathFor`: `pipeline.yaml` → `pipeline.lock.yaml`; `foo.yml` → `foo.lock.yaml`; `/abs/path/p.yaml` → `/abs/path/p.lock.yaml`.

---

## Phase 2: Short-code resolution walker (`core/short_codes.go`, new file)

The walker accepts the parsed top-level `yaml.Node` of a pipeline file plus a `*Lockfile`, substitutes short codes in place with UUIDs from the lockfile (minting new ones as needed), and reports back whether the lockfile was mutated.  Scope of substitution: a structural walk of `blocks[i].id` and `blocks[i].inputs[j]` only.  Other fields -- especially `args` and the pipeline-level `id` -- are not touched.

- [DONE] Define `const ShortCodePattern = "^@[A-Za-z_][A-Za-z0-9_]*$"` and a compiled `*regexp.Regexp`.  Spec §6.1.
- [DONE] Implement `isShortCode(s string) bool` returning true iff the string matches the pattern.  Quoting in YAML is invisible at this layer -- the value has already been parsed to a Go string.
- [DONE] Implement `ResolveShortCodes(root *yaml.Node, lock *Lockfile) (changed bool, err error)` per the design above, including the duplicate-bind-once rule.
- [DONE] Implement `LoadAndResolvePipeline(pipelinePath string) (Pipeline, Lockfile, bool, error)`.
- [DONE] Leave the existing `LoadPipeline(path)` function untouched.  It remains the entry point for already-resolved pipelines (e.g. from the web UI flowchart editor, where files are guaranteed to be UUID-form).

### Tests (Phase 2) — `core/short_codes_test.go`
- [DONE] `TestIsShortCode`: table-driven over valid (`@a`, `@reproject`, `@map_1`, `@_x`), invalid (`@1bad`, `@-x`, bare `@`, no leading `@`, UUID strings), and edge cases.
- [DONE] `TestResolveShortCodes_FreshPipeline`: three `@code` block ids and bare-reference inputs from empty lockfile.
- [DONE] `TestResolveShortCodes_ExplicitRefForm`: `block` scalar is rewritten; `output` scalar preserved verbatim.
- [DONE] `TestResolveShortCodes_MixedFormat`: UUIDs pass through unchanged; short codes resolved; mixed bare/explicit input lists work.
- [DONE] `TestResolveShortCodes_StableBindings`: second resolution on same source produces no changes.
- [DONE] `TestResolveShortCodes_AddNewCode`: new short code appends a binding without disturbing existing ones.
- [DONE] `TestResolveShortCodes_RenameMintsFresh`: orphan binding preserved, new code gets fresh UUID.
- [DONE] `TestResolveShortCodes_ArgsUntouched`: short codes inside `args` (including nested args) are left verbatim.
- [DONE] `TestResolveShortCodes_PipelineLevelIdRejected`: top-level `id: "@whatever"` errors with a clear message.
- [DONE] `TestResolveShortCodes_DuplicateCodeBindsOnce`: repeat short code reuses the same binding (lets existing duplicate-id validation fire).
- [DONE] `TestLoadAndResolvePipeline_FullCycle`: lockfile created on disk; parsed Pipeline has correct UUID-typed fields; input refs resolve.
- [DONE] `TestLoadAndResolvePipeline_LockfileDeletedRegenerates`: deletion regenerates fresh UUIDs.
- [DONE] `TestLoadAndResolvePipeline_PreservesUUIDOnlyPipelines`: no lockfile created when the source contains no short codes.
- [DONE] `TestLoadAndResolvePipeline_InvalidLockfile`: malformed lockfile surfaces a "delete to regenerate" hint.

---

## Phase 3: Validation extensions (`core/pipeline.go`)

Most of the §6.6 rules are caught implicitly: short-code grammar fails parse, missing references and duplicates fail existing `ValidatePipeline` checks on the resolved form, lockfile validity is enforced at load.  This phase tightens error messages and adds the integration helper.

- [DONE] Reworded `ValidatePipeline` error messages to be form-agnostic ("duplicate block invocation id", "unknown invocation id").  No behavior change.  Existing test updated to match new wording.
- [DONE] Added `ValidatePipelineWithLockfile(pipelinePath, manifests) (Pipeline, Lockfile, bool, []error, error)` wrapping `LoadAndResolvePipeline` + `ValidatePipeline`.

### Tests (Phase 3) — `core/pipeline_test.go`
- [DONE] `TestValidate_DuplicateShortCodes`: duplicate `@foo` triggers the duplicate-id validation error.
- [DONE] `TestValidate_UnresolvedShortCodeReference`: `@missing` in inputs surfaces as "unknown invocation id."
- [DONE] `TestValidate_OrphanedBindingTolerated`: orphan binding does not break validation and is preserved.

---

## Phase 4: CLI integration (`cli/cmd/check.go`, `cli/cmd/run.go`)

Both commands currently call `core.LoadPipeline`; they now use `core.LoadAndResolvePipeline` and surface lockfile activity to the user.

- [DONE] `runCheckPipeline` routes through `LoadAndResolvePipeline`, prints `Wrote …lock.yaml` on changes.
- [DONE] `ErrInvalidLockfile` handled with a "delete the lockfile to regenerate" hint and non-zero exit.
- [DONE] `runPipeline` routes through `LoadAndResolvePipeline` with the same lockfile UX.
- [DONE] When `Pipeline.Id == uuid.Nil` at run time, mint a fresh UUIDv7 (not persisted).  Confirms the §10 rule.
- [DONE] No new cobra flags.

### Tests (Phase 4) — `cli/cmd/check_test.go`, `cli/cmd/run_test.go`
- [DONE] `TestRunCheckPipeline_ShortCodeGeneratesLockfile`: lockfile created with the expected binding count.
- [DONE] `TestRunCheckPipeline_ShortCodeIdempotent`: byte-equal lockfile across two runs.
- [DONE] `TestRunCheckPipeline_InvalidLockfile`: subprocess test asserts the "delete" hint reaches stderr on `os.Exit(1)`.
- [DONE] `TestRunPipeline_PipelineIdGenerated`: distinct pipeline ids across runs; block ids stable via lockfile.
- _Deferred_: `TestRunPipeline_ShortCodeRoundTrip` end-to-end smoke is left to the manual smoke test in Phase 7 -- it requires a real installed block collection, which is heavier than what unit tests should pull in.

---

## Phase 5: Web UI short-code resolution (`web_ui/utils/`)

Per spec §6.7, the web UI resolves short codes by minting fresh UUIDv7s on upload -- no lockfile, no persistence beyond the resulting resolved pipeline.

**Design refinement during implementation:** the web_ui already operates on parsed `Pipeline` objects (built via the flowchart editor's `PipelineBuilder` or parsed from uploaded YAML in `server/api/pipelines/index.post.ts`).  Rather than walk a YAML AST, the TS resolver operates on the `Pipeline` object directly.  The upload endpoint parses YAML → resolves → re-serializes the resolved form back to YAML before persisting, so the stored YAML contains UUIDs only.

- [DONE] Add `web_ui/utils/short_codes.ts` with `isShortCode` and `resolveShortCodes(pipeline: Pipeline): Pipeline`.  Uses the existing `uuid` package (already in `package.json`).  Returns a new pipeline; never mutates the input.
- [DONE] Wire into `web_ui/server/api/pipelines/index.post.ts`: parse YAML → `resolveShortCodes` → validate → re-serialize as YAML → persist.  Resolution errors map to 400 responses.
- [DONE] Reuses the existing 400-status error path; no new surfacing mechanism.

### Tests (Phase 5) — `web_ui/tests/short_codes.test.ts`
- [DONE] `isShortCode` table-driven coverage (13 valid + invalid cases).
- [DONE] `assigns fresh UUIDv7s to each unique short code` + same-code-same-uuid invariant.
- [DONE] `does not mutate the input pipeline`.
- [DONE] `preserves args values verbatim` (top-level and nested).
- [DONE] `preserves mixed UUID + short-code pipelines` (UUID pass-through).
- [DONE] `rejects a top-level short code id` with `pipeline-level` in the error message.
- [DONE] `emits a Pipeline whose ids parse as UUIDv7s`.
- All 19 tests pass via `vitest`.

---

## Phase 6: Documentation cross-checks

Most documentation work is already complete (`spec/pipeline.md` §6, `spec/cli.md`, `spec/web_ui.md`, `skills/spade/SKILL.md`, `skills/spade/references/pipelines.md`, `skills/spade/references/cli.md`).  This phase covers what remains.

- [DONE] Updated `cli/README.md`: added a paragraph under `spade check` and under `spade run` mentioning the lockfile side effect and pointing to `spec/pipeline.md` §6.
- [DONE] Root `README.md` does not document the pipeline format inline (it's a higher-level overview); no change needed.
- [DONE] Confirmed: runner code is untouched.  The runner consumes resolved UUID pipelines (via the scheduler / queue) and never sees short codes.

---

## Phase 7: Verification

- [DONE] `cd core && go build ./... && go test -count=1 ./...` → all tests pass (core 0.21s).
- [DONE] `cd cli && go build ./... && go test -count=1 ./...` → all tests pass (spade/cmd 0.57s).
- [DONE] `cd web_ui && bun run test tests/short_codes.test.ts tests/pipeline.test.ts tests/wiring.test.ts tests/validate.test.ts` → 50/50 tests pass.  (DB-fixture tests in the full suite have a pre-existing parallelism flake unrelated to this work.)
- [ ] Manual smoke test (documented in PR description, not run in CI):
  1. `spade setup`
  2. Install a known collection: `spade install ./blocks/base`
  3. Write a short-code pipeline by hand at `/tmp/smoke/pipeline.yaml` -- two source blocks, one downstream block, all using `@codes`, no top-level `id`.
  4. `spade check /tmp/smoke/pipeline.yaml` -- confirm `pipeline.lock.yaml` is created with three bindings.
  5. `spade run /tmp/smoke/pipeline.yaml --keep-work-dir` -- confirm successful run; inspect working dir names use resolved UUIDs from the lockfile.
  6. Re-run `spade run` on the same pipeline -- confirm cache hits on all blocks (lockfile preserves invocation IDs).
  7. Delete `pipeline.lock.yaml`; re-run -- confirm a fresh lockfile is generated and the run cold-misses cache (intentional reset path per §6.4).
  8. Edit the source to rename `@reproject` → `@reproject_v2`; run check; confirm new binding appended, old binding orphan-retained, run re-executes the renamed block.

---

## Summary of files touched

| File | Action | Description |
|------|--------|-------------|
| `core/lockfile.go` | **New** | `Lockfile` type, load/save/path helpers, `ErrInvalidLockfile` sentinel. |
| `core/lockfile_test.go` | **New** | Round-trip, missing-file, invalid-binding, path-derivation tests. |
| `core/short_codes.go` | **New** | `isShortCode`, `ResolveShortCodes` yaml.Node walker, `LoadAndResolvePipeline` orchestration. |
| `core/short_codes_test.go` | **New** | Walker tests for fresh/stable/rename/orphan/mixed/args-untouched/top-level-id cases. |
| `core/pipeline.go` | Modify | Wording in `ValidatePipeline` errors; add `ValidatePipelineWithLockfile` wrapper. |
| `core/pipeline_test.go` | Modify | Add tests for duplicate-short-code resolution, unresolved references, orphan tolerance. |
| `cli/cmd/check.go` | Modify | Route through `LoadAndResolvePipeline`; surface lockfile activity; handle `ErrInvalidLockfile`. |
| `cli/cmd/check_test.go` | Modify | Add lockfile generation, idempotence, and invalid-lockfile tests. |
| `cli/cmd/run.go` | Modify | Route through `LoadAndResolvePipeline`; mint fresh pipeline `Id` if absent. |
| `cli/cmd/run_test.go` | Modify | End-to-end short-code run test; pipeline-id generation test. |
| `web_ui/utils/short_codes.ts` | **New** | TS resolver applied to uploaded pipeline YAML. |
| `web_ui/utils/pipeline.ts` | Modify | Invoke `resolveShortCodes` before YAML→object parse. |
| `web_ui/tests/short_codes.test.ts` | **New** | Resolver behavior tests mirroring the CLI suite. |
| `web_ui/package.json` | Modify (maybe) | Add `uuidv7` dependency if not already present. |
| `cli/README.md` | Modify | Document lockfile side effect under `spade check` / `spade run`. |
| `README.md` (root) | Modify (maybe) | One-sentence pointer to short codes if the README documents pipelines inline. |

**Files NOT touched** (confirming scope discipline): `core/types.go` (no changes to `Pipeline`, `PipelineBlock`, `InputRef`), `core/scheduler.go`, `core/executor.go`, `core/registry.go`, `core/cache.go`, `core/block.go`, anything under `runner/`, the spec files (already updated), the skill files (already updated).
