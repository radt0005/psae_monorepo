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

---

# Implementation Plan: Web UI ↔ Scheduler Contract Alignment

## Background

A contract audit against `spec/worker.md`, `spec/scheduler.md`, and `spec/web_ui.md` found three gaps between the web UI (`web_ui/`) and the Go scheduler (`server/`):

1. **Submission path uses the wrong transport.** `web_ui/server/api/runs/index.post.ts` publishes to a RabbitMQ queue named `user_submissions` after writing a `queued` row. Nothing reads `user_submissions`. `spec/worker.md` §Communication explicitly says the web UI → scheduler path uses a **Postgres outbox**, not the broker (RabbitMQ is worker ↔ scheduler only: `spade.jobs` / `spade.results`).

2. **No status feedback from scheduler to web UI.** The web UI has `PATCH /api/runs/:id` ready (with bearer-token auth via `WORKER_CALLBACK_SECRET`) but the Go scheduler never calls it. Runs submitted to the scheduler will be stuck in `running` forever from the web UI's perspective.

3. **Stale `Run` type in `web_ui/utils/types.ts`.** The `status` field is `"pending" | "running" | "error" | "completed"` — none match the database enum (`"queued" | "running" | "succeeded" | "failed" | "canceled"`) except `running`. The Vue pages define their own correct inline types and never import this, so there is no live breakage, but it is a trap for any future code that imports `Run` from `utils/types.ts`.

**Do these in order.** Each phase is independently deployable and testable. Phases 1 and 2 are web-UI-only. Phase 3 is Go-only and can be tested with `curl`. Phase 4 is Go-only and requires a shared PostgreSQL instance. Phase 5 ties them together with file/log forwarding.

**Shared infrastructure assumption.** The web UI and Go scheduler share one PostgreSQL instance (database `spade`) as shown in `web_ui/docker-compose.yml`. The scheduler's `SPADE_DATABASE_URL` and the web UI's `DATABASE_URL` both point at the same server. The scheduler reads from the web UI's `runs` table (different table, same database).

---

## Phase 1: Fix stale `Run` type (`web_ui/utils/types.ts`)

This is a one-line fix with no runtime impact.  Do it first so no future code inherits the wrong values.

- [DONE] In `web_ui/utils/types.ts` at line 94, change the `status` union on the `Run` type from:
  ```ts
  status: "pending" | "running" | "error" | "completed";
  ```
  to:
  ```ts
  status: "queued" | "running" | "succeeded" | "failed" | "canceled";
  ```
- [DONE] Confirm no other file in `web_ui/` imports `Run` from `utils/types.ts` with assumptions about the old status values.  (`grep -r 'from.*utils/types' web_ui/` and inspect each import site.)
- [DONE] `cd web_ui && bun run typecheck` passes with no new errors.

---

## Phase 2: Remove RabbitMQ from the web UI submission path

Per `spec/worker.md`: "the web UI ↔ scheduler path uses a Postgres outbox."  The `queued` row written by `runs/index.post.ts` is already the outbox entry.  RabbitMQ is not part of this path.

### 2.1 Simplify `web_ui/server/api/runs/index.post.ts`

- [ ] Delete the `import { publishJob } from "~/server/utils/rabbitmq"` line (currently line 7).
- [ ] Remove the `try/catch` block that calls `publishJob` and marks the run `failed` on enqueue error (currently lines 99–109).  The handler now ends with `return run;` immediately after `repo.create(...)`.
- [ ] Remove the comment block that describes the now-deleted enqueue step.
- [ ] Confirm the handler still validates the pipeline, resolves short codes, creates the DB row, and returns the run — unchanged from current behavior for those steps.

### 2.2 Delete `web_ui/server/utils/rabbitmq.ts`

- [ ] Delete `web_ui/server/utils/rabbitmq.ts`.  It exports `connectRabbitMQ` and `publishJob`; after 2.1 there are no remaining callers.
- [ ] `grep -r 'rabbitmq' web_ui/server/` to confirm no other file imports it.

### 2.3 Clean up environment config

- [ ] Remove `RABBITMQ_URL` and `RABBITMQ_QUEUE` from `web_ui/.env.example`.  `RABBITMQ_URL` is no longer needed by the web UI (it remains in the scheduler's env, which is a separate service).  Add a comment noting that RabbitMQ is used by the scheduler and worker, not the web UI.
- [ ] Remove `amqplib` from `web_ui/package.json` dependencies if it is only used by the deleted `rabbitmq.ts`.  Run `grep -r 'amqplib' web_ui/` first to confirm no other file imports it.
- [ ] `cd web_ui && bun install && bun run typecheck` passes.

### 2.4 Tests

- [ ] Run `cd web_ui && bun run test` — existing unit tests pass.
- [ ] Manually POST to `http://localhost:3000/api/runs` with a valid pipeline body and confirm the response contains a `queued` run with no 502 error.

---

## Phase 3: Scheduler → web UI status callback (Go)

The Go scheduler must call `PATCH <UI_BASE_URL>/api/runs/:id` whenever a pipeline transitions to `running`, `complete`, `failed`, or `cancelled`.  The PATCH endpoint already exists and is auth'd by a bearer token.

### 3.1 Add configuration flags (`server/cmd/spade-scheduler/app/run.go`)

- [ ] Add two new fields to the `Config` struct:
  ```go
  UIBaseURL        string  // base URL of the Nuxt web UI, e.g. http://localhost:3000
  UICallbackSecret string  // value of WORKER_CALLBACK_SECRET in the web UI .env
  ```
- [ ] Register them in `ParseFlags`:
  ```go
  fs.StringVar(&cfg.UIBaseURL, "ui-base-url", getenv("SPADE_UI_BASE_URL", ""), "Base URL of the Spade web UI for run callbacks")
  fs.StringVar(&cfg.UICallbackSecret, "ui-callback-secret", getenv("SPADE_UI_CALLBACK_SECRET", ""), "Bearer token for PATCH /api/runs/:id callbacks")
  ```
- [ ] When `UIBaseURL` is empty, log a warning at startup and skip all callback attempts (allows running the scheduler standalone without a web UI).

### 3.2 Implement the callback client (`server/api/callback.go`, new file)

- [ ] Create `server/api/callback.go` in package `api`.
- [ ] Define the request body struct:
  ```go
  type RunCallbackBody struct {
      Status string         `json:"status"`
      Error  string         `json:"error,omitempty"`
      Files  []CallbackFile `json:"files,omitempty"`
      Logs   []CallbackLog  `json:"logs,omitempty"`
  }

  type CallbackFile struct {
      Name     string `json:"name"`
      S3Key    string `json:"s3Key"`
      BlockID  string `json:"blockId,omitempty"`
      MimeType string `json:"mimeType,omitempty"`
  }

  type CallbackLog struct {
      BlockID string `json:"blockId,omitempty"`
      Stdout  string `json:"stdout,omitempty"`
      Stderr  string `json:"stderr,omitempty"`
  }
  ```
- [ ] Implement `PatchRunStatus(ctx context.Context, baseURL, secret, runID string, body RunCallbackBody) error`.
  - Marshals `body` to JSON.
  - Issues `PATCH <baseURL>/api/runs/<runID>` with `Content-Type: application/json` and `Authorization: Bearer <secret>`.
  - Returns nil on HTTP 200; returns a non-nil error (including the response status and body) on any other status.
  - Uses a 10-second timeout on the HTTP request context.
  - Does **not** retry — callers decide whether to retry.

### 3.3 Wire the callback into the engine (`server/engine/engine.go`)

- [ ] Add a `callbackFn func(pipelineID uuid.UUID, status store.PipelineStatus, errMsg string)` field to the `Engine` struct.  Default: `nil` (no-op).
- [ ] Add `func (e *Engine) SetCallback(fn func(uuid.UUID, store.PipelineStatus, string))` so the app layer can inject the callback without changing `engine.New`'s signature.
- [ ] In the engine's event-writing path, after writing each of:
  - `EventPipelineCompleted` → call `callbackFn(pipelineID, PipelineComplete, "")`
  - `EventPipelineFailed` → call `callbackFn(pipelineID, PipelineFailed, errorMessage)`
  - `EventPipelineCancelled` → call `callbackFn(pipelineID, PipelineCancelled, "")`
- [ ] Also fire the callback (with `PipelineRunning`) immediately after a pipeline is accepted and its first block is dispatched, so the web UI transitions from `queued` to `running` promptly.
- [ ] The callback is invoked synchronously in the engine's goroutine.  Failures are logged but do not affect engine state.

### 3.4 Wire up in `server/cmd/spade-scheduler/app/run.go`

- [ ] After constructing `eng` and before starting the HTTP server, if `cfg.UIBaseURL != ""`:
  ```go
  eng.SetCallback(func(id uuid.UUID, status store.PipelineStatus, errMsg string) {
      body := api.RunCallbackBody{Status: translateStatus(status), Error: errMsg}
      if err := api.PatchRunStatus(context.Background(), cfg.UIBaseURL, cfg.UICallbackSecret, id.String(), body); err != nil {
          logger.Warn("run callback failed", "run_id", id, "err", err)
      }
  })
  ```
- [ ] Implement `translateStatus(s store.PipelineStatus) string` in the same file:
  ```go
  func translateStatus(s store.PipelineStatus) string {
      switch s {
      case store.PipelineRunning:   return "running"
      case store.PipelineComplete:  return "succeeded"
      case store.PipelineFailed:    return "failed"
      case store.PipelineCancelled: return "canceled"
      default: return string(s)
      }
  }
  ```
  Note the spelling: Go uses `cancelled` (two `l`s); the web UI DB enum uses `canceled` (one `l`).

### 3.5 Tests (`server/api/callback_test.go`, new file)

- [ ] `TestPatchRunStatus_Success`: start an `httptest.Server` that asserts the correct method, path, Authorization header, and JSON body; respond 200; assert no error returned.
- [ ] `TestPatchRunStatus_Non200`: httptest responds 500; assert a non-nil error is returned containing the status code.
- [ ] `TestTranslateStatus`: table-driven over all four `PipelineStatus` values, including the `cancelled` → `canceled` spelling translation.
- [ ] `TestEngine_CallbackFiredOnComplete`: use an existing `engine_test` fixture; set a callback via `SetCallback`; submit a pipeline that completes; assert the callback is called with `PipelineComplete`.
- [ ] `TestEngine_CallbackFiredOnFailure`: same pattern, pipeline fails; assert callback called with `PipelineFailed`.
- [ ] `cd server && go test ./...` passes.

---

## Phase 4: Scheduler outbox poller — picking up `queued` runs from PostgreSQL (Go)

The scheduler polls the web UI's `runs` table for rows where `status = 'queued'` and submits each one via its own engine.  This is the Postgres outbox pattern specified in `spec/worker.md`.

### 4.1 Add configuration flags (`server/cmd/spade-scheduler/app/run.go`)

- [ ] Add one new field to `Config`:
  ```go
  UIDBUrl string  // PostgreSQL DSN for the web UI database (may be same as SPADE_DATABASE_URL)
  ```
- [ ] Register in `ParseFlags`:
  ```go
  fs.StringVar(&cfg.UIDBUrl, "ui-db-url", getenv("SPADE_UI_DB_URL", ""), "PostgreSQL DSN for the web UI database (outbox source)")
  ```
- [ ] If empty, fall back to `cfg.DatabaseURL` (single-DB topology; the `docker-compose.yml` default).
- [ ] Log the resolved DSN (masked, same as `maskDSN`) at startup.

### 4.2 Implement the outbox reader (`server/outbox/outbox.go`, new package)

- [ ] Create `server/outbox/outbox.go` in package `outbox`.
- [ ] Define `QueuedRun` struct matching the web UI's `runs` table columns the poller needs:
  ```go
  type QueuedRun struct {
      ID      string    // text primary key (UUIDv7 string)
      OwnerID string    // owner_id column
      YAML    string    // yaml column — the resolved pipeline YAML
  }
  ```
- [ ] Implement `FetchQueued(ctx context.Context, db *sql.DB, limit int) ([]QueuedRun, error)`:
  ```sql
  SELECT id, owner_id, yaml FROM runs WHERE status = 'queued' ORDER BY created_at ASC LIMIT $1
  ```
  Uses `database/sql` with the existing PostgreSQL driver already in the module graph.
- [ ] Implement `MarkRunning(ctx context.Context, db *sql.DB, id string) error`:
  ```sql
  UPDATE runs SET status = 'running', started_at = NOW(), updated_at = NOW()
  WHERE id = $1 AND status = 'queued'
  ```
  The `WHERE status = 'queued'` guard is the idempotency lock: a second concurrent poller picking up the same row will update 0 rows and can skip it.  Return `ErrAlreadyClaimed` (a sentinel defined in this package) when `rowsAffected == 0`.
- [ ] Implement `MarkFailed(ctx context.Context, db *sql.DB, id, errMsg string) error`:
  ```sql
  UPDATE runs SET status = 'failed', error = $2, finished_at = NOW(), updated_at = NOW() WHERE id = $1
  ```
  Used if the scheduler fails to parse or submit the pipeline.

### 4.3 Implement the poll loop (`server/outbox/poller.go`)

- [ ] Implement `Run(ctx context.Context, db *sql.DB, submit func(ctx context.Context, run QueuedRun) error, interval time.Duration)`.
  - On each tick: call `FetchQueued` (limit 10).
  - For each `QueuedRun`:
    1. Call `MarkRunning` — skip on `ErrAlreadyClaimed`.
    2. Call `submit(ctx, run)` — on error, call `MarkFailed` with the error message and log.
  - Sleep `interval` between polls.  Default interval: 5 seconds.
  - Returns when `ctx` is cancelled.
- [ ] The `submit` function is provided by the app layer (see 4.4), keeping the poller decoupled from the engine.

### 4.4 Wire up in `server/cmd/spade-scheduler/app/run.go`

- [ ] Open a `*sql.DB` connection to `cfg.UIDBUrl` (or fallback) alongside the existing `store.Store` connection.
- [ ] Construct the `submit` closure:
  ```go
  submit := func(ctx context.Context, run outbox.QueuedRun) error {
      var p core.Pipeline
      if err := yaml.Unmarshal([]byte(run.YAML), &p); err != nil {
          return fmt.Errorf("parse yaml: %w", err)
      }
      if err := eng.SubmitPipeline(ctx, &p, []byte(run.YAML), run.OwnerID); err != nil {
          if errors.Is(err, store.ErrAlreadyExists) {
              return nil // already submitted (e.g. scheduler restarted); not an error
          }
          return err
      }
      return nil
  }
  ```
- [ ] Launch the poller goroutine alongside the broker goroutine:
  ```go
  go func() {
      outbox.Run(ctx, uiDB, submit, 5*time.Second)
  }()
  ```
- [ ] Close `uiDB` in the shutdown path after context cancellation.

### 4.5 Tests (`server/outbox/outbox_test.go`)

- [ ] `TestFetchQueued`: insert three rows into an in-memory SQLite `runs` table (same schema), assert correct rows returned and ordering by `created_at`.
- [ ] `TestMarkRunning_Idempotency`: two concurrent calls to `MarkRunning` on the same row — assert exactly one returns nil and the other returns `ErrAlreadyClaimed`.
- [ ] `TestMarkFailed`: after `MarkFailed`, a subsequent `FetchQueued` returns no rows for that ID.
- [ ] `TestPollerSubmitsAndMarksRunning`: mock `submit` succeeds; assert `MarkRunning` was called; assert `MarkFailed` was not called.
- [ ] `TestPollerMarksFailedOnSubmitError`: mock `submit` returns an error; assert `MarkFailed` is called with a non-empty error message.
- [ ] `TestPollerSkipsAlreadyClaimed`: `MarkRunning` returns `ErrAlreadyClaimed`; assert `submit` is not called.
- [ ] `cd server && go test ./...` passes.

---

## Phase 5: File and log forwarding in the scheduler callback

After a pipeline completes, the scheduler has `InvocationRecord.LogsPath` (an S3 key) and `OutputHashesJSON` (a JSON map of output name → content hash) for each invocation.  The web UI needs file entries with `name` and `s3Key` to serve downloads.  This phase forwards that information as part of the completion callback.

### 5.1 Extract invocation records at pipeline completion (`server/engine/engine.go`)

- [ ] When firing the completion callback (`PipelineComplete` or `PipelineFailed`), call `store.LoadInvocations(ctx, pipelineID)` to get the full slice of `InvocationRecord` rows.
- [ ] Pass the slice to the `callbackFn` — add it as a third parameter or encapsulate in a new `CallbackPayload` struct:
  ```go
  type CallbackPayload struct {
      Status      store.PipelineStatus
      ErrorMsg    string
      Invocations []store.InvocationRecord
  }
  ```
- [ ] Update `SetCallback` signature accordingly; update `TestEngine_CallbackFiredOnComplete` in Phase 3 tests.

### 5.2 Build file entries from invocation records (`server/cmd/spade-scheduler/app/run.go`)

- [ ] In the callback closure, iterate `payload.Invocations` to build `[]api.CallbackFile`:
  - Parse `OutputHashesJSON` (a `map[string]string` of output name → hash) to enumerate output names.
  - The S3 key convention from `spec/worker.md` is `outputs/<invocation_id>/<output_name>/...`.  Construct the S3 key prefix as `outputs/<invocation_id>/<output_name>`.
  - Set `BlockID` to `invocation.ID` so the web UI can group files by block.
- [ ] Build `[]api.CallbackLog` from `InvocationRecord.LogsPath`:
  - Each invocation with a non-empty `LogsPath` produces one log entry with `BlockID = invocation.ID` and `Stdout` set to the S3 path (prefixed `s3://`).
  - Note: the web UI's `run_logs.stdout` column holds text content, not S3 paths. This is a schema mismatch.  For this phase, store the S3 path as the stdout value so files are at least discoverable.  A follow-up task can add a `logs_path` field to the `PATCH` body and to `run_logs` to handle this cleanly without polluting the text column.
- [ ] Pass the assembled `Files` and `Logs` slices in the `RunCallbackBody`.

### 5.3 Update `web_ui/server/api/runs/[id]/index.patch.ts`

- [ ] The handler currently accepts `logs: [{ blockId?, stdout?, stderr? }]` and inserts them as-is into `run_logs`.  No change needed for the initial pass — the S3 path stored in `stdout` will be visible in the log viewer as a path string, which is better than nothing.
- [ ] Open a follow-up issue to add `logsPath?: string` to the PATCH log entry and store it in a new `run_logs.logs_path` column.  Out of scope for this plan.

### 5.4 Tests

- [ ] `TestBuildFilesFromInvocations`: unit test the helper that converts `[]store.InvocationRecord` with `OutputHashesJSON` → `[]api.CallbackFile`; verify S3 key format and BlockID assignment.
- [ ] `TestBuildLogsFromInvocations`: one invocation with non-empty `LogsPath`; assert one log entry with the correct BlockID and path prefix.
- [ ] `cd server && go test ./...` passes.

---

## Phase 6: Configuration and deployment

- [ ] Add to `web_ui/.env.example`:
  ```
  # URL and secret the Go scheduler will use to POST back run status updates.
  # The web UI itself does not call the scheduler; the scheduler calls the web UI.
  # WORKER_CALLBACK_SECRET must match SPADE_UI_CALLBACK_SECRET in the scheduler's env.
  WORKER_CALLBACK_SECRET=replace-me-with-a-long-random-string
  ```
  (This key already exists in `.env.example` but the comment was misleading — update it to clarify the scheduler is the caller, not the worker binary directly.)

- [ ] Add to `server/README.md` a new "Environment variables" table covering `SPADE_UI_BASE_URL`, `SPADE_UI_CALLBACK_SECRET`, and `SPADE_UI_DB_URL` alongside the existing `SPADE_DATABASE_URL` and `SPADE_AMQP_URL` variables.

- [ ] Add a `spade-scheduler` service to `web_ui/docker-compose.yml` (or a root-level `docker-compose.yml`) so the full stack — PostgreSQL, MinIO, RabbitMQ, web UI, scheduler — starts with one command.  Wire the env vars listed above.  The scheduler and web UI services must share the same `DATABASE_URL` / `SPADE_DATABASE_URL` pointing at the same PostgreSQL instance.

- [ ] Confirm `web_ui/docker-compose.yml` still includes the RabbitMQ service — it remains necessary for the scheduler ↔ worker path even though the web UI no longer publishes to it.

---

## Phase 7: Verification

- [ ] **Type check**: `cd web_ui && bun run typecheck` — no errors.
- [ ] **Unit tests**: `cd web_ui && bun run test` — all pass.
- [ ] **Go build**: `cd server && go build ./...` — no errors.
- [ ] **Go tests**: `cd server && go test -count=1 ./...` — all pass, including new `outbox` and `api/callback` packages.
- [ ] **End-to-end smoke test** (requires running `docker compose up -d`):
  1. Start the full stack: `docker compose up -d` (postgres, minio, rabbitmq, web UI, scheduler).
  2. Register a user via the web UI.
  3. In the pipeline editor, build a minimal one-block pipeline (e.g. using a block that is already installed on the local test worker).
  4. Submit the pipeline ("Run").
  5. Confirm the web UI immediately shows the run as `queued`, then transitions to `running` within ~5 seconds (outbox poller picks it up).
  6. When the pipeline completes, confirm the run transitions to `succeeded` or `failed` (callback fired).
  7. Confirm output files appear in the run detail view (file entries forwarded in callback).
  8. Cancel a running pipeline from the web UI — confirm the run transitions to `canceled`.
  9. Restart the scheduler mid-run — confirm the run resumes and completes (Postgres outbox + engine recovery path).

---

## Summary of files touched

| File | Action | Description |
|------|--------|-------------|
| `web_ui/utils/types.ts` | Modify | Fix `Run.status` union to match the actual `runStatusEnum`. |
| `web_ui/server/api/runs/index.post.ts` | Modify | Remove `publishJob` call; handler ends after `repo.create`. |
| `web_ui/server/utils/rabbitmq.ts` | **Delete** | No longer called; web UI no longer publishes to RabbitMQ. |
| `web_ui/package.json` | Modify | Remove `amqplib` dependency if it has no other callers. |
| `web_ui/.env.example` | Modify | Remove `RABBITMQ_URL` / `RABBITMQ_QUEUE`; update `WORKER_CALLBACK_SECRET` comment. |
| `server/api/callback.go` | **New** | `PatchRunStatus`, `RunCallbackBody`, `CallbackFile`, `CallbackLog`. |
| `server/api/callback_test.go` | **New** | Tests for HTTP callback client and status translation. |
| `server/outbox/outbox.go` | **New** | `FetchQueued`, `MarkRunning`, `MarkFailed`, `ErrAlreadyClaimed`. |
| `server/outbox/poller.go` | **New** | `Run` — the poll loop that drives the outbox reader. |
| `server/outbox/outbox_test.go` | **New** | Unit tests for outbox reader and poll loop. |
| `server/engine/engine.go` | Modify | Add `callbackFn`, `SetCallback`, and `CallbackPayload`; fire callback on state transitions. |
| `server/cmd/spade-scheduler/app/run.go` | Modify | Wire `UIBaseURL`, `UICallbackSecret`, `UIDBUrl` config; construct callback closure; start outbox poller goroutine. |
| `server/README.md` | Modify | Document new environment variables. |
| `web_ui/docker-compose.yml` (or root) | Modify | Add `spade-scheduler` service with correct env wiring. |

**Files NOT touched:** `runner/`, `core/`, `cli/`, `web_ui/server/api/runs/[id]/index.patch.ts` (already correct), `web_ui/server/db/schema/runs.ts` (already correct), `spec/` files.
