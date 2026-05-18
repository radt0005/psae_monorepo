# Scheduling Server — Implementation Plan

This plan brings the `server/` directory from its current "Hello, World!"
state to a working scheduling server that satisfies the specifications in
`../spec/`.

The scheduling server is the **scheduler** described in `../spec/scheduler.md`
and the **App Platform service** described in `../spec/hosting.md` §3.  It is
the central coordination process: pipelines come in from the web UI, get
written to PostgreSQL, are turned into block invocations, dispatched to
workers via the `spade.jobs` RabbitMQ queue, and have their results consumed
from the `spade.results` queue.

## Key decision: reuse `../core/` and `../runner/broker/`

The `../core/` Go module already implements the multi-tenant scheduling
primitives the server needs.  The `../runner/broker/` package already
implements the RabbitMQ transport with the exact queue names, durability,
and ack semantics the spec requires.

| Capability | Existing package | Server uses |
|---|---|---|
| Pipeline / block / manifest / input-ref types | `core/types.go` | Re-exports `core.Pipeline`, `core.BlockManifest`, `core.InputRef`, `core.PipelineBlock` |
| `WorkerAssignment` / `WorkerResult` wire structs | `core/types.go` | Built into outgoing `spade_runner.Job` payloads; parsed from incoming results |
| `spade_runner.Job` envelope (assignment + pipeline + manifests) | `runner/types.go` | Server publishes this exact JSON to `spade.jobs` |
| DAG, ready-set tracking, map context, map/reduce expansion | `core/scheduler.go` (`SinglePipelineScheduler`, `MultiTenantScheduler`, `MapContext`) | The server's in-memory engine is a thin wrapper around `core.MultiTenantScheduler` |
| Pipeline validation | `core/pipeline.go` (`ValidatePipeline`) | Run on `POST /pipelines` before persistence |
| RabbitMQ queue declaration, durable consume/publish, ack-after-confirm | `runner/broker/` (`Conn`, `JobConsumer`, `ResultPublisher`) | Server uses the *opposite ends*: a `JobPublisher` on `spade.jobs` and a `ResultConsumer` on `spade.results` |
| Reconnect loop with backoff | `runner/broker/reconnect.go` | Reused by the server-side broker driver |

**The server must not reimplement any of the above.**  It adds only:

1. PostgreSQL persistence for pipeline DAGs, invocation history, expansion
   manifests, and result idempotency state (the durable source of truth per
   `scheduler.md` §State Management).
2. A `spade.jobs` publisher and `spade.results` consumer (the inverse of
   the runner's two transport halves).
3. The engine layer that drives `core.MultiTenantScheduler`: pulls ready
   invocations, builds `spade_runner.Job` payloads, hands them to the
   broker, applies incoming results back to the scheduler, and persists
   each state change.
4. An HTTP API (Echo v5; the existing `main.go` skeleton already imports it)
   for the web UI to submit pipelines, browse status, and cancel runs.
5. The `spade-scheduler` binary: signal handling, startup recovery
   from PostgreSQL, broker reconnect, graceful drain.

If `core/` is missing something the server needs, the fix goes upstream
into `core/` rather than being duplicated here.  This plan lists those
gaps in Phase 3.

---

## Phase 1: Module Plumbing

### Phase 1.1: `go.mod` setup

- [DONE] Change `module server` to `module spade_server` so the import path is unambiguous when cross-referenced from siblings.
- [DONE] Add `require core v0.0.0` with a `replace core => ../core` directive so the local sibling module is picked up (matches the pattern used in `runner/go.mod`).
- [DONE] Add `require spade_runner v0.0.0` with a `replace spade_runner => ../runner` directive so the server can re-use the `spade_runner.Job` envelope type and `runner/broker` package without copying.
- [DONE] Add direct requires the server needs that are not already inherited transitively:
  - [DONE] `github.com/google/uuid` — UUID handling in API and engine layer.
  - [DONE] `gopkg.in/yaml.v3` — parsing pipelines submitted as YAML bodies.
  - [DONE] `gorm.io/gorm` — same ORM the core registry uses, for the new PostgreSQL persistence layer.
  - [DONE] `gorm.io/driver/postgres` — Postgres driver for GORM.
  - [DONE] `github.com/Azure/go-amqp` — already pulled in transitively by the rabbitmq-amqp-go-client through runner/broker; list it directly because the server consumer also references `amqp.Error`.
  - [DONE] `github.com/stretchr/testify` — test assertions.
- [DONE] Keep existing `github.com/labstack/echo/v5` and `github.com/labstack/echo/v5/middleware` as they already satisfy the HTTP layer requirement.
- [DONE] Run `go mod tidy` after each phase that adds imports so the lockfile stays clean.

### Phase 1.2: Build smoke

- [DONE] Confirm `go build ./...` in `server/` succeeds after the module rename and `replace` directives. A minimal `main.go` that imports `core` and `spade_runner/broker` is sufficient to verify the import paths resolve.
- [DONE] Add a `.gitignore` for the built `spade-scheduler` binary, `*.db` / `*.db-wal` / `*.db-shm` artifacts from any tests that touch SQLite (the registry mirror tests), and a local `tmp/` workspace.

---

## Phase 2: Package Layout

Create these packages under `server/`.  Each is described in detail in its
own phase further down.

- [DONE] **`store/`** — PostgreSQL persistence: pipelines, invocations, expansion manifests, idempotency markers. Phase 4.
- [DONE] **`broker/`** — RabbitMQ transport for the scheduler side: `JobPublisher` (writes `spade.jobs`) and `ResultConsumer` (reads `spade.results`). Phase 5.
- [DONE] **`engine/`** — The in-memory scheduling engine. Wraps `core.MultiTenantScheduler`, applies results to it, pulls ready invocations, builds outgoing `spade_runner.Job` payloads, and calls into `store/` on every state change. Phase 6.
- [DONE] **`api/`** — Echo HTTP handlers for the web UI: submit pipelines, list pipelines, get pipeline status, get invocation status, cancel pipelines, fetch logs. Phase 7.
- [DONE] **`cmd/spade-scheduler/`** — The binary main. Wires config, opens the store, dials the broker, starts the engine and the HTTP server, handles signals. Phase 8.
- [DONE] **`testutil/`** — Fakes and fixtures shared across tests: in-memory `store` substitute, fake broker that pairs with the runner's fake, sample pipelines reused from `../runner/testutil/fixtures/`.

Packages **not** created here because they already live elsewhere:

- Pipeline / block / manifest types → `core/types.go`.
- Dependency-graph / validation → `core/pipeline.go`.
- Scheduling primitives → `core/scheduler.go`.
- AMQP queue declaration and reconnect → `runner/broker/`.

---

## Phase 3: Gaps in `core/` and `runner/` to fix upstream

Before implementing the server, walk the spec against `core/` and `runner/`
and catalog anything missing.  When something is missing, the plan entry is
to **add it upstream** rather than build it inside `server/`.

- [DONE] **3.1 Scheduler-facing dispatch hook on `MultiTenantScheduler`.** Today `(*MultiTenantScheduler).Next` returns the next ready `BlockInvocation` but exposes no callback or channel for "a new block just became executable." The server needs to drain ready blocks promptly without busy-polling. *Upstream fix (core):* add `(*MultiTenantScheduler).Drain() []BlockInvocation` that atomically returns every currently-executable invocation across all pipelines, plus an optional `ReadyCh() <-chan struct{}` that is signaled whenever `Update` produces a state transition that makes at least one new block ready. Keep `Next` as-is for tests.
- [DONE] **3.2 Per-pipeline status snapshot.** The HTTP API needs to answer "what is the state of every block in this pipeline?" without reaching into the scheduler's private maps. *Upstream fix (core):* add `(*SinglePipelineScheduler).Snapshot() PipelineSnapshot` returning a value type with: pipeline ID, per-block status (`pending`, `executable`, `in-flight`, `complete`, `error`), the `MapContext` for any map blocks (including the resolved expansion items so the API can show fan-out progress), and the cancellation flag.  And `(*MultiTenantScheduler).Snapshot(pipelineID) (PipelineSnapshot, bool)`.
- [DONE] **3.3 In-flight tracking on `MultiTenantScheduler`.** `Next` records the invocation in `CurrentExecutions` but `Update` removes it by `invocationId` — the mapping from result back to invocation must survive map fan-out (where the in-flight key is `<UUID>.<index>`, not the bare UUID). *Upstream fix (core):* key `CurrentExecutions` by `InvocationID() string` instead of `uuid.UUID`, and have `Next` and `Update` agree on that key.  This is a small refactor but unblocks correct map handling end-to-end.
- [DONE] **3.4 Idempotency helper.** The spec requires the scheduler to be idempotent when consuming results (`scheduler.md` §State Management and `worker.md` §Result reporting). *Upstream fix (core):* expose `(*MultiTenantScheduler).IsAlreadyProcessed(invocationID string) bool` so the server's result consumer can early-return a duplicate without mutating state.  The implementation can simply check the per-pipeline scheduler's `CompletedBlocks`.
- [DONE] **3.5 Pipeline reconstruction on startup.** The server must reconstruct the in-memory scheduler from PostgreSQL on restart. The minimum API needed is a way to replay results into the scheduler in deterministic order. *Upstream fix (core):* add `(*MultiTenantScheduler).Rehydrate(pipeline Pipeline, completedResults []BlockInvocationResult) error` that runs `AddPipeline` then applies each completed result via `Update` in order. This avoids the server poking at internal fields.
- [DONE] **3.6 Map block downstream manifest population.** When a map expansion creates N invocations, the worker needs each downstream block's manifest in the outgoing `Job.Manifests` map. *Upstream fix (core):* ensure `BuildJobForInvocation(inv BlockInvocation, p Pipeline, manifests map[string]BlockManifest) spade_runner.Job` (or similar helper) is exported.  Today the worker builds this inversely from a `Job` it already received; the server has to build it forward.  If putting this in `core/` creates an import cycle (because `spade_runner` imports `core`), put it in `runner/` instead as `spade_runner.BuildJob(...)`.
- [DONE] **3.7 Scheduler-side `Job` builder in `runner/`.** Following on 3.6, add `spade_runner.BuildJob(assignment core.WorkerAssignment, pipeline core.Pipeline, manifests map[string]core.BlockManifest) Job` to `runner/types.go` so the server can construct outgoing jobs symmetrically with the worker's `InvocationFromJob`.
- [DONE] **3.8 Result decoding helper.** The worker publishes `core.WorkerResult` JSON. The scheduler needs to map that back to `core.BlockInvocationResult` for the scheduler engine. *Upstream fix (core):* add `core.WorkerResultToInvocationResult(r WorkerResult) BlockInvocationResult` so the server does not duplicate the field-mapping logic.

Every checkbox below in Phases 4–10 assumes the gaps above have been
addressed first.

---

## Phase 4: PostgreSQL Persistence (`store/`)

The scheduler's durable state is summarized in `scheduler.md` §State
Management: pipeline DAGs and invocation result history live in PostgreSQL,
and outstanding work lives in RabbitMQ.  In-memory bookkeeping is a cache
rebuilt on startup.  This phase implements the PostgreSQL side.

### Phase 4.1: Schema

- [DONE] Define a GORM model `Pipeline` matching the persisted form of a pipeline:
  - `ID` (uuid, primary key) — matches `core.Pipeline.Id`.
  - `Name` (string).
  - `Version` (string).
  - `Description` (string, nullable).
  - `YAML` (text) — the canonical resolved (UUID-form) pipeline YAML as submitted. Stored verbatim so reruns and audit are exact.
  - `Status` (string: `running`, `complete`, `failed`, `cancelled`).
  - `SubmittedAt` (timestamp).
  - `CompletedAt` (timestamp, nullable).
  - `SubmitterUserID` (string, nullable — populated when authenticated submissions land; for the first cut we just store whatever the API hands us).
- [DONE] Define a GORM model `BlockInvocationRecord` matching the persisted form of each invocation:
  - `ID` (string, primary key) — uses the invocation ID string form (`<UUID>` for non-mapped, `<UUID>.<index>` for mapped). This is the natural idempotency key per `worker.md` §Result reporting.
  - `PipelineID` (uuid, indexed).
  - `BlockID` (uuid, indexed) — matches the `id` field in the pipeline block (the UUID without any map suffix).
  - `BlockName` (string) — the block-type name, e.g. `raster.reproject`.
  - `MapIndex` (int, nullable).
  - `Status` (string: `pending`, `ready`, `dispatched`, `complete`, `error`, `map`, `reduce`).
  - `DispatchedAt` (timestamp, nullable).
  - `CompletedAt` (timestamp, nullable).
  - `ExitCode` (int).
  - `LogsPath` (string).
  - `ErrorMessage` (string).
  - `OutputHashesJSON` (text) — serialized `map[string]string` from the worker's `WorkerResult.OutputHashes`.
  - `ExpansionJSON` (text, nullable) — serialized expansion manifest when this invocation was a map block.
- [DONE] Define a GORM model `PipelineEvent` for the audit / replay log:
  - `ID` (uint, primary key).
  - `PipelineID` (uuid, indexed).
  - `InvocationID` (string, indexed, nullable).
  - `EventType` (string: `submitted`, `block_dispatched`, `block_completed`, `block_failed`, `pipeline_completed`, `pipeline_cancelled`).
  - `PayloadJSON` (text).
  - `CreatedAt` (timestamp, indexed).
- [DONE] Add a `gorm:"uniqueIndex"` constraint on `BlockInvocationRecord.ID` so that any double-write attempt fails fast — this is the idempotency guard.

### Phase 4.2: Store interface and implementation

- [DONE] Define a `Store` interface in `store/store.go` with the methods used by `engine/`. Designing the interface (not just the GORM-backed struct) keeps the engine testable with an in-memory fake:
  - [DONE] `InsertPipeline(ctx, pipeline core.Pipeline, yamlBody []byte) error`
  - [DONE] `LoadPipeline(ctx, id uuid.UUID) (core.Pipeline, error)`
  - [DONE] `ListPipelines(ctx, filter ListFilter) ([]PipelineSummary, error)`
  - [DONE] `UpdatePipelineStatus(ctx, id uuid.UUID, status string) error`
  - [DONE] `UpsertInvocation(ctx, rec BlockInvocationRecord) error` — INSERT … ON CONFLICT DO UPDATE keyed on `ID` so dispatch and completion writes are idempotent.
  - [DONE] `LoadInvocations(ctx, pipelineID uuid.UUID) ([]BlockInvocationRecord, error)`
  - [DONE] `LoadActivePipelinesForRestart(ctx) ([]ActivePipeline, error)` — returns each pipeline that is still in status `running`, with its parsed `core.Pipeline` and the complete list of `BlockInvocationRecord` it has so far. Used by `engine.Recover` on startup.
  - [DONE] `AppendEvent(ctx, evt PipelineEvent) error`
- [DONE] Implement the GORM-backed `*PgStore` against PostgreSQL. Open the connection via `gorm.Open(postgres.Open(dsn), ...)`.
- [DONE] Implement an in-memory `*MemStore` in `store/memstore.go` for unit tests.  It does not need ACID semantics, only the interface contract.
- [DONE] Auto-migrate the three models on first connect (`db.AutoMigrate(&Pipeline{}, &BlockInvocationRecord{}, &PipelineEvent{})`). Wire this into the `Open` constructor so tests get a fresh schema.
- [DONE] Add a `Close()` method on `*PgStore` that calls `(*sql.DB).Close()` on the underlying connection pool.

### Phase 4.3: Idempotency at the storage layer

- [DONE] In `UpsertInvocation`, the *complete* and *error* status writes must use `ON CONFLICT (id) DO UPDATE SET ... WHERE invocations.status NOT IN ('complete', 'error')`. This is the first-result-wins guarantee from `worker.md` §Result reporting at the database level — a duplicate result arriving after the original cannot overwrite the original's exit code, logs, or expansion.
- [DONE] Write a test that calls `UpsertInvocation` twice with conflicting `ExitCode` values for an already-complete invocation and asserts that the second call leaves the row untouched.

### Phase 4.4: Configuration

- [DONE] Read the Postgres DSN from `SPADE_DATABASE_URL` (defaulting to `postgres://localhost/spade?sslmode=disable` for local dev), matching the App Platform secret-injection pattern used by the worker config.
- [DONE] On startup, refuse to start if the DSN is unreachable. The spec (`scheduler.md` §State Management) is explicit that PostgreSQL is the source of truth; starting without it would silently lose state.

---

## Phase 5: RabbitMQ Transport (`broker/`)

The server is the *opposite end* of the runner's transport: it publishes
jobs and consumes results, instead of the runner's consume-jobs and
publish-results pattern.

### Phase 5.1: Reuse the existing connection layer

- [DONE] Import `spade_runner/broker` and reuse its `Conn`, `Dial`, `ensureQueues`, `Close`, `QueueJobs`, `QueueResults`, and reconnect loop.  These already satisfy the wire-level contract in `worker.md` §Communication.
- [DONE] The runner's `broker.Conn` already exposes `NewResultPublisher(QueueResults)`.  Add (in the runner package, not the server) `NewJobPublisher(ctx)` returning a publisher bound to `QueueJobs`, mirroring the existing `NewResultPublisher`. This keeps queue-name knowledge in one place.  Note this is a gap to fix upstream in `runner/broker/amqp.go`; the server's plan entry is "use it".

### Phase 5.2: Job publisher

- [DONE] Add a `JobPublisher` adapter in `server/broker/` that wraps the runner's underlying publisher.
- [DONE] Method `Publish(ctx, job spade_runner.Job) error` — marshals the `Job` to JSON, publishes with `persistent: true`, blocks until publisher confirms (the runner's existing publisher already returns only after the broker accepts; preserve that contract).
- [DONE] On publish failure, return the underlying error untouched so the engine layer can decide whether to retry, fail, or reconnect.
- [DONE] Idempotency: include a deterministic `message-id` derived from the invocation ID (e.g. `inv:<InvocationID>`) so RabbitMQ's optional dedup plugin can filter duplicate redrives. This is defensive — the protocol already tolerates duplicates via worker-side idempotency — but cheap.

### Phase 5.3: Result consumer

- [DONE] Add a `ResultConsumer` in `server/broker/` that wraps the runner's underlying consumer interface, but bound to `QueueResults` instead of `QueueJobs`.  The runner's `amqpConsumer` is generic enough that the server can either reuse it directly or copy the few lines that bind to the queue name.
- [DONE] Method `Next(ctx) (ResultDelivery, error)` returning a struct with:
  - [DONE] `Result core.WorkerResult` — parsed JSON.
  - [DONE] `RawBody []byte` — for logging on parse failure.
  - [DONE] `Ack(ctx) error` — settle as accepted.
  - [DONE] `Nack(ctx, requeue bool) error` — settle as rejected.
- [DONE] Method `Close(ctx) error`.

### Phase 5.4: Reconnect

- [DONE] Reuse the runner's `broker.Run(ctx, ReconnectConfig, handler)` reconnect-with-backoff loop. The server's handler runs *two* concurrent goroutines per connection: one driving the result consumer into the engine, and one feeding the engine's outbound dispatch queue into the job publisher. If either errors, the handler returns and the reconnect loop redials.

### Phase 5.5: Tests

- [DONE] Reuse the runner's `broker/fake.go` in-memory fake.  The server's engine tests need a paired fake that delivers `core.WorkerResult` messages on demand and records published `spade_runner.Job` messages for assertion.

---

## Phase 6: Scheduling Engine (`engine/`)

The engine is the thin layer between `core.MultiTenantScheduler` and the
broker + store.  It owns the in-memory state-machine lifecycle, applies
broker events to the scheduler, drains executable blocks, builds outgoing
jobs, and persists every transition.

### Phase 6.1: Engine type

- [DONE] Define `engine.Engine` with fields:
  - [DONE] `sched *core.MultiTenantScheduler` — owns the per-pipeline schedulers.
  - [DONE] `store store.Store` — durable persistence.
  - [DONE] `pub broker.JobPublisher` — outgoing dispatches.
  - [DONE] `manifests core.ManifestProvider` — resolves block name → `BlockManifest`. See Phase 6.2 for what this is.
  - [DONE] `logger *slog.Logger`.
  - [DONE] An internal mutex guarding `sched` mutations.
- [DONE] `New(store, pub, manifests, logger) *Engine`.
- [DONE] `Close()` — flushes any pending dispatches and stops the dispatch goroutine.

### Phase 6.2: Manifest provider

- [DONE] Define `core.ManifestProvider` interface with one method: `Lookup(blockName, version string) (core.BlockManifest, error)`. (This is a small upstream addition in `core/`; the registry already has the data.)
- [DONE] Production implementation: query the Plugin Registry metadata mirror in PostgreSQL (`registry.md` §10).  For the first cut, the simpler path is to query the per-worker block index API once at startup and cache results in memory — the registry mirror integration can come later.
- [DONE] Test implementation: a `map[string]core.BlockManifest` wrapper that returns manifests from a fixed in-memory map.

### Phase 6.3: Pipeline submission

- [DONE] `Engine.SubmitPipeline(ctx, p core.Pipeline, yamlBody []byte) error`:
  1. Validate the pipeline via `core.ValidatePipeline(p, manifests)`. If invalid, return the validation error before touching any state.
  2. Within a single transaction, call `store.InsertPipeline` and append a `submitted` event.
  3. Under the engine mutex, call `sched.AddPipeline(p)`.
  4. For map-containing pipelines, populate `sched.Schedulers[p.Id].Manifests` from the manifest provider (the existing `SinglePipelineScheduler` already uses this map for `IdentifyMapContexts`).
  5. Trigger a dispatch sweep (Phase 6.4) so source blocks go out immediately.
  6. Return nil.

### Phase 6.4: Dispatch loop

- [DONE] Run a dedicated goroutine `Engine.dispatchLoop(ctx)` that:
  1. Waits on a `ready` signal channel (signaled after `SubmitPipeline`, after every consumed result, and after engine startup recovery).
  2. Pulls every currently-executable invocation from `sched.Drain()` (or repeatedly calls `sched.Next(workerID)` with a placeholder worker ID, depending on which upstream API in Phase 3.1 lands).
  3. For each invocation, builds a `core.WorkerAssignment` (filling `InvocationID`, `BlockName`, `PipelineID`, `WorkDir`, `Args`, `Inputs`) using the corresponding `core.PipelineBlock`.
  4. Wraps the assignment, the pipeline (loaded from `store`), and the manifests for the invocation's block and every direct dependency into a `spade_runner.Job` via `spade_runner.BuildJob` (Phase 3.7).
  5. Publishes the job via `pub.Publish`.
  6. On success: `store.UpsertInvocation` with status `dispatched`, `DispatchedAt = now()`; append a `block_dispatched` event.
  7. On publish failure: log, push the invocation back onto the engine's internal pending queue (do NOT mutate the scheduler), and break out so the reconnect loop redials. Once reconnected, the dispatch will be retried.
- [DONE] The dispatch loop must be reentrant: signaling `ready` while the loop is mid-iteration must result in at least one more sweep, never a missed wakeup. Use a single-slot `chan struct{}` with non-blocking send to coalesce signals.

### Phase 6.5: Result intake

- [DONE] Run a dedicated goroutine `Engine.resultLoop(ctx, consumer broker.ResultConsumer)` that:
  1. Pulls the next `ResultDelivery` from the consumer.
  2. If the JSON failed to parse (`RawBody` populated but `Result.InvocationID` empty), log and Nack without requeue. This matches the worker's malformed-payload handling in `runner/worker/loop.go`.
  3. If `sched.IsAlreadyProcessed(result.InvocationID)` returns true: log "duplicate", Ack, continue. This is the first-result-wins guarantee from `worker.md` §Result reporting.
  4. Translate `core.WorkerResult` → `core.BlockInvocationResult` (Phase 3.8 helper).
  5. Under the engine mutex: `sched.Update(invocationID, invocationResult)`. The scheduler internally handles map expansion, reduce readiness, and pipeline halts on `ExecutionStatusError` (see `core/scheduler.go` lines 68–121).
  6. Persist the change: `store.UpsertInvocation` with the terminal status; append a `block_completed` or `block_failed` event. For map results, also persist the expansion manifest JSON.
  7. If `sched` reports the pipeline complete (all schedulers' pending+executable empty and no cancellation): `store.UpdatePipelineStatus(complete)` and append a `pipeline_completed` event.
  8. If the result was a failure: the scheduler already halts the pipeline; reflect that with `store.UpdatePipelineStatus(failed)` and a `pipeline_cancelled` event.
  9. Signal the dispatch loop via the `ready` channel so any newly-ready blocks go out immediately.
  10. Ack the delivery.
- [DONE] If `store.UpsertInvocation` fails after the scheduler has been updated, do NOT ack; Nack without requeue and rely on broker redelivery + the idempotency guard to converge.  Log loudly because it indicates DB trouble.

### Phase 6.6: Map and Reduce

- [DONE] `core.SinglePipelineScheduler` already handles `ExecutionStatusMap` (`core/scheduler.go` lines 194–272) and `ExecutionStatusReduce` (lines 276–322) internally. The engine just has to pass results with the right `Status` through.
- [DONE] When a result has `Status == ExecutionStatusMap`, the engine receives the expansion manifest in `result.Expansion`. It must:
  1. Persist the expansion as part of the map block's `BlockInvocationRecord`.
  2. For each of the N mapped invocations the scheduler creates, persist a fresh `BlockInvocationRecord` with status `pending` and `MapIndex` set, so the database mirrors the in-memory fan-out.
- [DONE] Verify by integration test (Phase 9.4) that a 3-item expansion produces three persisted rows with IDs `<UUID>.0`, `<UUID>.1`, `<UUID>.2`, each transitions through `dispatched` → `complete`, and the reduce block runs exactly once after all three.

### Phase 6.7: Cancellation

- [DONE] `Engine.CancelPipeline(ctx, id uuid.UUID) error`:
  1. Under the engine mutex, call `sched.CancelPipeline(id)`. The existing `core.SinglePipelineScheduler.CancelPipeline` clears pending + executable and sets `Cancelled = true`.
  2. `store.UpdatePipelineStatus(id, "cancelled")`.
  3. Append a `pipeline_cancelled` event.
- [DONE] In-flight invocations are NOT recalled. Per `worker.md` §Communication, workers ack the job only after completion; an in-flight invocation will run to completion, publish its result, and be discarded by the result loop's idempotency check (the scheduler has no pending entry for it once cancelled).
- [DONE] Write a test that submits a 2-block pipeline, dispatches the first block, cancels the pipeline, has the worker publish the (now-orphaned) result, and asserts the engine ignores it without state corruption.

### Phase 6.8: Restart-tolerant recovery

This is the heart of `scheduler.md` §State Management.

- [DONE] `Engine.Recover(ctx) error`:
  1. Calls `store.LoadActivePipelinesForRestart(ctx)` to fetch every pipeline still in status `running` with its full invocation history.
  2. For each, calls `sched.Rehydrate(p, completedResults)` (Phase 3.5) so the in-memory state matches the persisted truth.
  3. Identifies invocations persisted as `dispatched` but not yet `complete`/`error`. These are in-flight from the previous incarnation — at most one is in the broker's hands per the prefetch=1 contract. The result loop's idempotency check handles whatever comes back.
  4. Triggers a dispatch sweep so any newly-ready blocks (which were ready when the previous scheduler crashed but had not been published) are now sent.
- [DONE] Recovery must be called *before* the dispatch loop and result loop start. The binary main (Phase 8) enforces this order.

### Phase 6.9: Pipeline status snapshot

- [DONE] `Engine.PipelineStatus(ctx, id uuid.UUID) (PipelineStatus, error)`:
  - Returns the snapshot from `sched.Snapshot(id)` (Phase 3.2), merged with the persisted pipeline header (name, version, submitted_at) and the latest completion timestamp.
- [DONE] This is what the HTTP `GET /pipelines/{id}` handler calls.

### Phase 6.10: Worker registration

- [DONE] The current spec uses competing-consumer semantics over a single queue: workers do not register with the scheduler, and the scheduler does not pick a worker (`worker.md` §Communication: "competing consumers, no scheduler-side worker tracking"). Therefore the server **does not** call `sched.AddWorker` or maintain a worker registry. Note: the `core.MultiTenantScheduler.AddWorker` API exists but is currently unused; do not wire it up in this phase.
- [DONE] The optional `preferred_worker_id` field on `Job` is reserved for future affinity routing (`worker.md` §Job dispatch). The engine must NOT populate it in this implementation but the `Job` struct already accepts it.

---

## Phase 7: HTTP API (`api/`)

The HTTP API is what the web UI calls to submit pipelines and check on
them.  The Echo v5 framework is already wired in `main.go`; this phase
fleshes out the routes.

### Phase 7.1: Echo setup

- [DONE] Move the existing `main.go` Echo skeleton into `cmd/spade-scheduler/main.go` (Phase 8).  Leave a stub `main.go` at the package root that just calls into the binary.
- [DONE] Build an `api.Server` struct in `server/api/server.go` that owns:
  - [DONE] `*echo.Echo` — the router.
  - [DONE] `*engine.Engine` — the scheduling engine.
  - [DONE] `store.Store` — direct read access for endpoints that don't need engine logic.
  - [DONE] `*slog.Logger`.
- [DONE] Register routes in `api.Server.Routes()`.

### Phase 7.2: Endpoints

The endpoints below are the minimum set required to support the user-visible
features in `web_ui.md`: submit pipelines, view results, browse past
pipelines, cancel runs.

- [DONE] `GET /` → 200 `OK` health check. Used by the App Platform health-check probe (`hosting.md` §3.2).
- [DONE] `GET /healthz` → 200 if the DB ping and broker connection are both healthy; 503 otherwise. Used by liveness/readiness probes.
- [DONE] `POST /pipelines` — Submit a new pipeline:
  - Request body: YAML or JSON of `core.Pipeline`. If the `id` field is missing, mint a fresh UUIDv7. (Short-code resolution does not happen here — `pipeline.md` §6 requires the CLI to resolve short codes before submission, and `web_ui.md` §"Uploading Short-Code-Form Pipelines" requires the UI to resolve them at upload time. The server sees only UUID-form pipelines.)
  - Validate via `core.ValidatePipeline`. Return 400 with the full list of validation errors if any.
  - Call `engine.SubmitPipeline`.
  - Return 201 with `{ id, name, version, status: "running" }`.
- [DONE] `GET /pipelines` — List pipelines: filter by `status`, `submitter`, paginated. Returns `PipelineSummary[]`.
- [DONE] `GET /pipelines/{id}` — Pipeline detail: header + every block's current status (from `engine.PipelineStatus`). Returns 404 if not found.
- [DONE] `DELETE /pipelines/{id}` — Cancel pipeline: calls `engine.CancelPipeline`. Returns 204.
- [DONE] `GET /pipelines/{id}/invocations` — Full per-invocation list including map indices, exit codes, dispatched/completed timestamps, log paths.
- [DONE] `GET /invocations/{invocation_id}` — Single invocation by string ID (`<UUID>` or `<UUID>.<index>`). Returns 404 if not found.
- [DONE] `GET /invocations/{invocation_id}/logs` — Streams stdout/stderr. The worker uploads logs to object storage (`worker.md` §Storage Model and `hosting.md` §6.2); the handler resolves the LogsPath from the invocation record and returns the upload URL or streams from the object store. For the first cut, return the LogsPath and let the UI fetch directly from Spaces with a signed URL — defer the streaming proxy to a later iteration unless the UI design forces it sooner.

### Phase 7.3: Auth (deferred but unblocked)

- [DONE] All endpoints take an optional `X-Spade-User-Id` header for now; record it on the pipeline submission so the audit trail is correct.  Better Auth integration (`hosting.md` §7) is its own deliverable. Leaving the field nullable now means it can be backfilled without a schema change.

### Phase 7.4: Tests

- [DONE] Table-driven handler tests using `httptest.NewRecorder` + Echo's built-in test helpers. Pair each handler with at least: a happy-path case, a validation-failure case, and a not-found case.

---

## Phase 8: Binary (`cmd/spade-scheduler/main.go`)

The binary glues everything together.

- [DONE] Define a `config` struct with the same shape as the worker's: `AMQPURL`, `DatabaseURL`, `HTTPAddr` (default `:1323`, matching the existing skeleton), `LogLevel`, `ShutdownGraceSec`.
- [DONE] `parseFlags()` reads each with `flag.*Var`, defaulting to the matching `SPADE_*` env var. This matches the worker's `parseFlags` exactly so operators have one mental model.
- [DONE] Logger: `slog.NewJSONHandler(os.Stderr, ...)` at the configured level, matching the worker.
- [DONE] Startup sequence (order matters):
  1. Open `store.NewPgStore(dsn)` — auto-migrates the schema.
  2. Dial RabbitMQ via the runner's `broker.Dial`.
  3. Construct `engine.New(store, pub, manifests, logger)`.
  4. Call `engine.Recover(ctx)` to rebuild in-memory state. If this fails, exit non-zero — the server cannot safely continue.
  5. Start `engine.dispatchLoop(ctx)` and `engine.resultLoop(ctx, consumer)` as goroutines.
  6. Start the HTTP server via `echoServer.Start(httpAddr)` in its own goroutine.
  7. Wait on a `signal.NotifyContext` for SIGINT/SIGTERM.
- [DONE] Shutdown sequence:
  1. Cancel the root context.
  2. Wait up to `ShutdownGraceSec` for in-flight work (dispatches in the publisher, results being applied) to drain.
  3. Close the HTTP server, the broker connection, and the store, in that order.

The whole binary should mirror `runner/cmd/spade-worker/main.go` in
structure so operators only have one shutdown contract to learn.

---

## Phase 9: Tests

### Phase 9.1: Unit tests

- [DONE] `store/`: GORM model round-trip, idempotency guard (Phase 4.3), Active-pipeline-recovery fixture.
- [DONE] `broker/`: pair the runner's fake publisher with a fake server-side consumer in a single test that confirms a `spade_runner.Job` round-trips JSON correctly.
- [DONE] `engine/`:
  - [DONE] Source-block dispatch fires on submission.
  - [DONE] Dependency completion triggers downstream dispatch.
  - [DONE] Map block expansion creates N persisted records.
  - [DONE] Reduce block runs exactly once after N mapped invocations complete.
  - [DONE] Duplicate result is ignored (idempotency).
  - [DONE] Block failure halts the pipeline and updates DB status to `failed`.
  - [DONE] Cancellation drops pending blocks and ignores orphaned results.
  - [DONE] Recovery from a half-dispatched pipeline resumes correctly.
- [DONE] `api/`: handler tests per Phase 7.4.

### Phase 9.2: Integration test

- [DONE] Spin up a real Postgres via `testcontainers-go` (or an environment variable-gated Postgres URL for CI) and the runner's broker fake. Submit a 3-block linear pipeline, drive the worker's fake responses, and assert: each block transitioned through `dispatched → complete`, the pipeline reached `complete`, and the persisted invocation history matches expectations.
- [DONE] Repeat for a map/reduce pipeline: map → fan-out 3 → reduce, asserting both fan-out and gather semantics.
- [DONE] Repeat for a failure scenario: 3-block pipeline where block 2 returns `ExecutionStatusError`, asserting block 3 is never dispatched and the pipeline status is `failed`.
- [DONE] Repeat for a restart scenario: submit a pipeline, dispatch the first block, hard-kill the engine, instantiate a fresh engine pointed at the same DB and broker fake, and assert recovery picks up where the previous one left off.

### Phase 9.3: End-to-end smoke

- [DONE] Optional `make smoke` target that runs the scheduler against a real RabbitMQ (CloudAMQP-equivalent local Docker) and a real `spade-worker` from `../runner/`, processing a tiny no-op block. Gated behind an env var; not required to pass CI by default.

---

## Phase 10: Operational Hardening

These items are required to satisfy the hosting plan (`hosting.md` §3.2: App
Platform restart-tolerance) and the failure modes documented in
`scheduler.md` §Error Handling.

- [DONE] Connection retries: the binary must survive a transient Postgres
  reconnect (`gorm` already retries individual queries; the binary should
  also retry the initial `Open` once with backoff).
- [DONE] The RabbitMQ reconnect loop already lives in `runner/broker/reconnect.go`;
  ensure the server's handler is idempotent across reconnects.
- [DONE] Add a Prometheus-style `/metrics` endpoint exposing: current pipelines
  by status, dispatch lag (time between block becoming ready and being
  published), and result-consume lag. Defer detailed dashboarding to the
  hosting-out-of-scope items (`hosting.md` §11).
- [DONE] Container image: a multi-stage Dockerfile that builds the binary with
  `go build -o spade-scheduler ./cmd/spade-scheduler` and copies it into a
  `gcr.io/distroless/static` base. The DO Container Registry expects this
  per `hosting.md` §6.4.
- [DONE] Documentation: a `README.md` covering local dev (Postgres + RabbitMQ
  via `docker compose`), env vars, and the API surface. Marks the deferred
  Better Auth integration explicitly so the next developer doesn't waste
  time wondering about it.

---

## Completion Criteria

The implementation is complete when:

1. `go build ./...` succeeds and `go test ./...` passes in `server/`.
2. The binary boots against a fresh Postgres and an empty RabbitMQ broker
   without manual schema setup.
3. Submitting a 3-block linear pipeline via `POST /pipelines` causes the
   three blocks to be dispatched, executed by a real `spade-worker` from
   `../runner/`, and the pipeline reaches `complete` status in the database.
4. Killing the binary mid-pipeline and restarting it resumes the pipeline
   from PostgreSQL with no manual intervention and no double-execution.
5. A map/reduce pipeline correctly fans out, gathers, and reduces.
6. A failing block correctly halts its pipeline without affecting any
   concurrently-running pipeline.