# Spade Worker (Runner) — Implementation Plan

This plan builds the **Spade worker binary** (`spade-worker`) and its
supporting package layer in this folder.  The runner is the RabbitMQ-driven
process described in `../spec/worker.md`.

## Key decision: use `../core/`

The `../core/` Go module already implements the bulk of the worker's
internal primitives (per `core/README.md`):

| Capability | `core/` file | Runner uses |
|---|---|---|
| Pipeline / block / manifest / input-ref types | `types.go` | Re-exports `core.Pipeline`, `core.BlockManifest`, `core.InputRef`, etc. |
| Worker ↔ scheduler message structs | `types.go` (`WorkerAssignment`, `WorkerResult`) | Serialized directly as RabbitMQ payloads |
| Dependency graph + input resolution + pipeline validation | `pipeline.go` | `core.ResolveInputs` called inside the worker when building `invocation.yaml` |
| Invocation directory creation, `params.yaml` writing, symlinking (including map/reduce cases and broadcast), output hashing, language detection, entrypoint resolution | `block.go` | `core.CreateBlockDirectory`, `core.WriteParamsYAML`, `core.SetupInputSymlinks`, `core.SetupBroadcastInputs`, `core.CollectOutputs`, `core.ComputeContentHash`, `core.ResolveEntrypoint` |
| SQLite block registry (GORM + WAL + `0600` + verify + rebuild) | `registry.go` | `core.OpenRegistry`, `core.LookupBlock`, `core.VerifyBlock` |
| Full subprocess execution under `isolate`, with stdout/stderr log capture, map-expansion handling, and `network: true` opt-in | `executor.go` | `core.Execute` is the single entry point the worker invokes per job |
| Cache keys, store, restore | `cache.go` | Used as a post-execution optimization only if enabled via config |
| Single/multi-tenant scheduler (DAG, map/reduce) | `scheduler.go` | **Not used by the runner** — the runner is a *worker*, not a scheduler |

**The runner must not reimplement any of the above.**  It adds only:

1. The RabbitMQ transport (`spade.jobs` consumer, `spade.results` publisher)
2. A thin orchestration wrapper that converts a `core.WorkerAssignment`
   delivered over the broker into a `core.Execute` call and its result back
   into a `core.WorkerResult` publish-then-ack
3. The `spade-worker` binary (signal handling, reconnect loop, config)
4. An embeddable single-invocation API so `spade run` (local single-instance
   mode, per `../spec/cli.md`) can reuse the same orchestration without a
   broker

If `core` is missing something the worker needs (see §3 "Gaps in `core/`"
below), the fix goes upstream into `core/` rather than being duplicated
here.  The runner keeps its surface area minimal.

---

## Phase 1 — Module Plumbing

- [ ] 1.1 Update `runner/go.mod`:
  - [ ] Change `module spade_runner` → `module spade_runner` (keep as-is; the module name is already fine).
  - [ ] Add `require core v0.0.0` and a `replace core => ../core` directive so the local sibling module is picked up.
  - [ ] Add direct requires that the runner needs:
    - [ ] `github.com/rabbitmq/rabbitmq-amqp-go-client` — already present (AMQP 1.0 client for `spade.jobs` / `spade.results`).
    - [ ] `github.com/google/uuid` — already pulled in transitively through `core`; list it directly since the runner uses `uuid.UUID` in signatures.
    - [ ] `gopkg.in/yaml.v3` — same rationale (used to parse block manifests inside the worker when the assignment references a block by name).
    - [ ] `github.com/stretchr/testify` — test assertions.
  - [ ] Run `go mod tidy` after each phase that adds imports.
- [ ] 1.2 Confirm `go build ./...` in `runner/` succeeds with the `replace core => ../core` directive in place (an empty smoke `main.go` under `cmd/spade-worker/` is sufficient to verify the import resolves).
- [ ] 1.3 Add a `.gitignore` for the built `spade-worker` binary and `*.db` / `*.db-wal` / `*.db-shm` artifacts dropped by SQLite tests.

## Phase 2 — Package Layout

Create these packages under `runner/`:

- [ ] 2.1 `broker/` — RabbitMQ consumer/publisher for the two durable queues.
- [ ] 2.2 `worker/` — orchestration: one-`WorkerAssignment`-to-one-`WorkerResult` using `core.Execute`.  Exported so the CLI can reuse it for local single-instance mode (`spade run`).
- [ ] 2.3 `cmd/spade-worker/` — the worker binary.
- [ ] 2.4 `testutil/` — fakes (fake AMQP conn, fake core executor) + fixture paths shared across tests.

Packages **not** created here because they already live in `core/`:
`types/`, `registry/`, `fs/`, `resolve/`, `exec/`, `sandbox/`, `mapblk/`,
`hash/`, `logs/`.  See the "Gaps" discussion in §3 for any exceptions.

## Phase 3 — Gaps in `core/` (fix upstream, not here)

Before implementing the runner, walk the spec against `core/` and catalog
anything missing.  When something is missing, the plan entry is to **add
it to `core/`** rather than build it inside `runner/`.  Each item below
says where the upstream change should land.

- [ ] 3.1 **Manifest lookup by block name.**  `core.BlockRegistry` returns a
  `BlockRegistryEntry` but not the parsed manifest.  `core.Execute` needs a
  `BlockManifest`.  Today the runner would have to re-read the manifest
  from `<InstalledPath>/blocks/<BlockName>.yaml`.  **Upstream fix (core):**
  add `core.LoadManifestForEntry(entry BlockRegistryEntry) (BlockManifest, error)`
  that reads the YAML from the entry's installed path.  This avoids the
  runner hardcoding the on-disk layout.
- [ ] 3.2 **Pipeline-dir resolution in the worker.**  `core.Execute` takes a
  `pipelineDir` parameter.  The `WorkerAssignment` type only has `WorkDir`.
  Clarify in the runner's orchestration: `pipelineDir = WorkDir`.  If the
  `WorkerAssignment` has no `WorkDir` (older scheduler), derive one as
  `<SPADE_WORK_ROOT>/<PipelineID>/`.  No `core` change needed, but
  document the invariant in a code comment so future `core` changes don't
  silently break it.
- [ ] 3.3 **Dependency manifests during input resolution.**  `core.ResolveInputs`
  requires `dependencies map[uuid.UUID]BlockManifest`.  The scheduler does
  *not* currently ship those with the assignment.  Two options:
  - (a) the scheduler populates them in `WorkerAssignment` (preferred, needs
    a field added to `core.WorkerAssignment`), or
  - (b) the worker walks the dependency IDs in `assignment.Inputs`, looks up
    each dependency block's name via the pipeline (requires adding the
    pipeline to the message), and loads the manifests from the registry.
  **Upstream fix (core):** add `Pipeline *Pipeline` **and** `DepManifests
  map[uuid.UUID]BlockManifest` fields to `core.WorkerAssignment`.  This is
  the minimum the worker needs to do input resolution and symlinking
  without round-tripping to the scheduler.
- [ ] 3.4 **Result publication payload parity.**  `core.WorkerResult` already
  has `Status`, `Error`, `Expansion`, `OutputHashes`.  The spec
  (`../spec/worker.md`) also asks for an **exit code** in the `Result`
  message.  **Upstream fix (core):** add `ExitCode int` to
  `core.WorkerResult`.  `core.Execute` currently folds the exit code into
  `Error`; change it to populate `ExitCode` directly.
- [ ] 3.5 **Log references.**  The spec says results carry "log references".
  Today `core.Execute` writes `logs/stdout.log` and `logs/stderr.log`
  inside the invocation directory.  **Upstream fix (core):** add
  `LogsPath string` to `core.WorkerResult`, populated with the absolute
  path to the invocation's `logs/` directory.
- [ ] 3.6 **Registry default path helper.**  `core.OpenRegistry` takes an
  explicit path.  The worker binary needs a default of
  `$HOME/.spade/registry.db`.  **Upstream fix (core):** add
  `core.DefaultRegistryPath() string`.  Keeps the CLI and the worker
  binary using the same default.
- [ ] 3.7 **Assignment → invocation conversion.**  `WorkerAssignment` has
  `InvocationID string` (possibly `<uuid>.<idx>`) but `BlockInvocation.Id`
  is a `uuid.UUID` + optional `MapIndex int`.  **Upstream fix (core):**
  add `core.InvocationFromAssignment(a WorkerAssignment, p Pipeline) (BlockInvocation, error)`
  that parses `<uuid>[.<idx>]`, resolves the referenced `PipelineBlock` to
  get `Arguments`/`Inputs`, and returns a fully populated `BlockInvocation`.
  The runner will call this exactly once per job.

Each gap above is an individual core-side change tracked as part of this
plan.  Do not proceed with Phase 4+ until the gap fixes are merged into
`core/` and `go mod tidy` in `runner/` picks them up.

## Phase 4 — `broker/` — RabbitMQ transport

AMQP 1.0 via `github.com/rabbitmq/rabbitmq-amqp-go-client` (already in
`go.sum`).  Implements §Communication of `../spec/worker.md`.

- [ ] 4.1 `broker/conn.go`:
  - [ ] `type Conn struct { ... }` holding the underlying AMQP connection and session.
  - [ ] `Dial(ctx context.Context, url string) (*Conn, error)` — connects with a 30s heartbeat; returns `*Conn` on success.
  - [ ] `(*Conn).Close(ctx) error` — closes sender/receiver links, then the session, then the connection.
  - [ ] `(*Conn).EnsureQueues(ctx) error` — idempotently declares `spade.jobs` and `spade.results` as `durable: true`.  Safe to call on every reconnect.
- [ ] 4.2 `broker/consumer.go`:
  - [ ] `type JobConsumer struct { ... }` wrapping an AMQP receiver on `spade.jobs`.
  - [ ] `NewJobConsumer(c *Conn, prefetch int) (*JobConsumer, error)` — opens the receiver with `prefetch_count == 1` unless overridden.  Per the spec, prefetch is 1 in production; the flag exists only for testing.
  - [ ] `type Delivery struct { Assignment core.WorkerAssignment; Ack func(ctx) error; Nack func(ctx, requeue bool) error; RawBody []byte }`.
  - [ ] `(*JobConsumer).Next(ctx) (Delivery, error)` — blocks until the next message; unmarshals the JSON body into `core.WorkerAssignment`.  If unmarshal fails, the raw body is preserved in `Delivery.RawBody` and `Delivery.Ack/Nack` are still usable so the binary can reject the malformed message.
- [ ] 4.3 `broker/publisher.go`:
  - [ ] `type ResultPublisher struct { ... }` wrapping an AMQP sender on `spade.results`.
  - [ ] `NewResultPublisher(c *Conn) (*ResultPublisher, error)` — sender with `durable: true` on every message.
  - [ ] `(*ResultPublisher).Publish(ctx, result core.WorkerResult) error` — marshals to JSON, sends, **waits for the broker publisher confirm before returning**.  This guarantees the result is durably enqueued before the job is acked.
- [ ] 4.4 `broker/reconnect.go`:
  - [ ] `Run(ctx, url string, handler func(*Conn) error) error` — wraps a dial-plus-work loop with exponential backoff (100ms → 30s cap) on any transient error.  The worker binary's main loop uses this to stay connected across broker restarts.
- [ ] 4.5 Tests (`broker/*_test.go`):
  - [ ] Unit test using a small hand-rolled fake that implements the AMQP client interface; verifies: prefetch is set to 1, queues are declared durable, `Publish` waits for confirm before returning.
  - [ ] Integration test behind `-tags integration` that uses `github.com/testcontainers/testcontainers-go` to boot a real RabbitMQ container, round-trips an assignment → result pair, and asserts that the result body is exactly the JSON-marshaled form of `core.WorkerResult`.

## Phase 5 — `worker/` — Orchestration

Pure library layer.  One function, one job.  The binary wraps this with a
broker loop; the CLI wraps it with a local single-instance loop.

- [ ] 5.1 `worker/worker.go`:
  - [ ] `type Worker struct { Registry *core.BlockRegistry; WorkRoot string; CacheDir string; UseCache bool }`.
  - [ ] `New(registry *core.BlockRegistry, workRoot string, opts ...Option) *Worker` with functional options for cache dir, cache enable/disable, and a test-injectable "now" clock.
  - [ ] `(*Worker).Run(ctx context.Context, a core.WorkerAssignment) (core.WorkerResult, error)`.
  - [ ] `Run` steps (each mapped to `core/` primitives):
    1. Registry lookup — `w.Registry.LookupBlock(a.BlockName, "")` returns `*BlockRegistryEntry`.  On miss → `WorkerResult{Status: ExecutionStatusError, Error: "block not installed: "+a.BlockName}`, return **nil** error (block failure, not infra failure).
    2. Manifest load — `core.LoadManifestForEntry(entry)` (added in Phase 3.1).  Same failure-mode convention on error.
    3. Invocation build — `core.InvocationFromAssignment(a, *a.Pipeline)` (added in Phase 3.7).  Populates the `BlockInvocation`, including `MapIndex` for `<uuid>.<idx>` forms.
    4. Pipeline dir — resolve to `a.WorkDir` (Phase 3.2 invariant).  Ensure it exists (`os.MkdirAll` with `0777` to match `core.CreateBlockDirectory`'s mode convention).
    5. Input resolution — `core.ResolveInputs(pipelineBlock, a.DepManifests, manifest)` (uses fields added in Phase 3.3).
    6. Cache check (if `w.UseCache`): compute input hashes from the resolved deps' output directories, call `core.ComputeCacheKey`, then `core.CacheLookup`.  On hit, `core.CacheRestore` into the invocation's `outputs/` and short-circuit to a successful `WorkerResult` with the cached output hashes.
    7. Symlink setup — if `core.Execute` handles `WriteParamsYAML` + `SetupInputSymlinks` internally today, skip this; otherwise call them explicitly.  Revisit after a code-read pass during implementation.
    8. Execute — `core.Execute(inv, pipelineDir, manifest, *entry, w.Registry)`.  Propagate the returned `BlockInvocationResult`.
    9. Map finalization — if `result.Status == ExecutionStatusMap`, the expansion is already on `result.Expansion`; no extra work.
    10. Cache store (if `w.UseCache` and the run succeeded): `core.CacheStore(key, outputsDir, w.CacheDir)`.
    11. Convert `BlockInvocationResult` → `core.WorkerResult`: map status verbatim, copy `Expansion`, move the output list to `OutputHashes` (requires the `core.CollectOutputs` return value, which `core.Execute` already computes — exposed via `BlockInvocationResult.Outputs`).  Populate `ExitCode` and `LogsPath` from Phase 3.4 / 3.5 upstream changes.
    12. Return the `WorkerResult` with nil error.
  - [ ] `Run` returns a **non-nil error only for infrastructure failures** (registry handle closed, pipeline dir unwritable, manifest load fails with a system error rather than a parse error).  Every block-level failure (missing block, hash mismatch, non-zero exit, bad expansion) is reported as `WorkerResult{Status: ExecutionStatusError, Error: ...}` with `err == nil` so the broker loop can ack and publish.
- [ ] 5.2 Split-brain guarantee.  The worker owns the `isolate` sandbox lifecycle indirectly (through `core.RunBlockSubprocess` called by `core.Execute`), so canceling `ctx` must propagate into the subprocess.  If `core.Execute` does not yet accept a context (it currently does not, per the signature above), file that as Phase 3.8 and fix upstream: `core.Execute(ctx, ...)`.  Without context propagation, `spade-worker` cannot honor `SIGTERM` during a long block.  **This gap blocks Phase 5.1 step 8.**
- [ ] 5.3 Tests (`worker/worker_test.go`, `worker/integration_test.go`):
  - [ ] Happy path with a `testutil`-built Go fixture block that writes `outputs/message/message.txt` and exits 0 → `Status == Complete`, `OutputHashes` populated.
  - [ ] Block name missing from registry → `Status == Error`, nil error returned.
  - [ ] Hash mismatch (tamper the binary after install) → `Status == Error` with "content hash mismatch" in `Error`, nil error returned.
  - [ ] Block exits non-zero → `Status == Error`, stderr tail in `Error`, log files exist on disk at `LogsPath`.
  - [ ] Map block happy path → `Status == Map`, `Expansion` carries the items.
  - [ ] Map block exits 0 but writes no expansion → `Status == Error`, nil error.
  - [ ] Registry handle closed → **non-nil** error returned (infra failure).
  - [ ] Cache hit short-circuits execution (assert `isolate` was not invoked by using a fake executor hook).

## Phase 6 — `cmd/spade-worker/` — Binary

- [ ] 6.1 `cmd/spade-worker/main.go`:
  - [ ] Flags via stdlib `flag` (the CLI uses cobra; the worker binary stays minimal):
    - `--amqp-url`, env `SPADE_AMQP_URL`, default `amqp://guest:guest@localhost:5672/`.
    - `--work-root`, env `SPADE_WORK_ROOT`, default `$HOME/.spade/work`.
    - `--registry`, env `SPADE_REGISTRY`, default `core.DefaultRegistryPath()` (added in Phase 3.6).
    - `--cache-dir`, env `SPADE_CACHE_DIR`, default `$HOME/.spade/cache`.  Empty string disables caching.
    - `--prefetch` default `1` (spec) — flag exists solely for testing.
    - `--shutdown-grace-sec` default `60` for the graceful-drain window.
  - [ ] Startup:
    1. Parse flags.
    2. Probe `isolate --version`; log and `os.Exit(1)` if missing (per §Security of `worker.md`).
    3. `core.OpenRegistry(cfg.RegistryPath)`; fail hard on error.
    4. `broker.Dial`; `broker.EnsureQueues`.
    5. `worker.New(registry, workRoot, opts...)`.
  - [ ] Main loop (inside `broker.Run(ctx, url, func(c *broker.Conn) error { ... })`):
    1. `consumer := broker.NewJobConsumer(c, cfg.Prefetch)`.
    2. `publisher := broker.NewResultPublisher(c)`.
    3. Loop while `ctx.Err() == nil`:
       - `delivery, err := consumer.Next(ctx)`.
       - `result, runErr := worker.Run(ctx, delivery.Assignment)`.
       - If `runErr != nil` (infrastructure failure), log it, `delivery.Nack(ctx, false /* don't requeue */)` — the broker redelivers to another worker per §Error Handling case 2.  Do **not** publish a `Result`.  Continue the loop.
       - Otherwise `publisher.Publish(ctx, result)` (blocks until confirm).  Then `delivery.Ack(ctx)`.
    4. On `ctx` cancel, return from the handler so `broker.Run` unwinds cleanly.
  - [ ] Signals:
    - `SIGTERM`/`SIGINT` → cancel the root context.  In-flight job finishes (publishes + acks) unless it exceeds `cfg.ShutdownGraceSec`, at which point the context deadline elapses, `core.Execute`'s context cancellation propagates to the `isolate` subprocess (depends on Phase 5.2), and the job message is left unacked for redelivery.
  - [ ] Structured logging via `log/slog` with JSON handler.  Required fields: `time`, `level`, `msg`, `invocation_id` (when known), `block_name` (when known), `pipeline_id` (when known).
- [ ] 6.2 Build target: `go build -o spade-worker ./cmd/spade-worker`.  Document in `runner/README.md`.
- [ ] 6.3 Tests (`cmd/spade-worker/main_test.go`):
  - [ ] Harness-driven test: pass a fake `broker` conn + fake `core.Execute` through interface seams.  Assert:
    - Successful run: `Publish` is called before `Ack`.
    - Infra failure: `Nack(requeue=false)` is called and **no** `Publish` happens.
    - Signal mid-flight: the in-flight job completes if within the grace window; otherwise the context cancel propagates and no ack/publish happens.
  - [ ] End-to-end integration test (`-tags integration`) that boots RabbitMQ + installs a Go fixture block + sends an assignment and asserts a result is received.

## Phase 7 — `testutil/` — Shared fixtures

- [ ] 7.1 `testutil/fixtures/hello-go/` — minimal Go collection with one `hello` block that reads `params.yaml`, writes `outputs/message/message.txt`, and exits 0.  Includes `go.mod`, `blocks/hello.yaml`, and a `main.go` dispatcher.
- [ ] 7.2 `testutil/fixtures/broken-go/` — a Go block that exits 1 and writes a stderr message used by failure-path tests.
- [ ] 7.3 `testutil/fixtures/map-files/` — Go block with `kind: map` that enumerates the files in `inputs/source/` and writes a deterministic `outputs/manifest/expansion.yaml`.  Used to exercise `core`'s map path through the runner.
- [ ] 7.4 `testutil/harness.go`:
  - [ ] `SpadeHome(t *testing.T) string` — creates a temp `$HOME`-style tree, builds each fixture via `go build`, registers them in a fresh `core.BlockRegistry`, and returns the root so tests can pass it as `WorkRoot`.
  - [ ] `FakeAMQP()` — in-memory implementation of the `broker.Conn` / `Consumer` / `Publisher` interfaces so unit tests do not require Docker.
  - [ ] `PublishJob(t, queue, a core.WorkerAssignment)` — helper used by integration tests.

## Phase 8 — Cross-Spec Verification

After the code is written, walk each spec bullet and confirm coverage.
Each item is a checklist the reviewer can tick off:

- [ ] 8.1 `../spec/worker.md` §File System: invocation dir with `params.yaml`, `inputs/`, `outputs/`, `logs/` — covered by `core.CreateBlockDirectory` called from `core.Execute`, reached via `worker.Run`.
- [ ] 8.2 `../spec/worker.md` §Block Registry (SQLite, `0600`, WAL, rebuildable) — `core.OpenRegistry` + `core.BlockRegistry.RebuildFromFilesystem`; the worker binary merely consumes the registry.
- [ ] 8.3 `../spec/worker.md` §Block Lookup — `core.LookupBlock` + `core.LoadManifestForEntry` (new in Phase 3.1) called from `worker.Run` step 1–2.
- [ ] 8.4 `../spec/worker.md` §Execution per language — `core.ResolveEntrypoint` covers Rust, Go, Python (via `uv run`), TypeScript (Bun), R.
- [ ] 8.5 `../spec/worker.md` §Security — `core.RunBlockSubprocess` uses `isolate` with default-deny network, `--share-net` for `network: true`, `--mem`, `--time`, `--dir=<workDir>:rw`, and language-specific binds.  Memory records already note: use isolate, not go-landlock.
- [ ] 8.6 `../spec/worker.md` §Logging — `core.RunBlockSubprocess` captures `stdout`/`stderr` into `<work>/logs/`.  The runner adds `LogsPath` to the published `WorkerResult` (Phase 3.5).
- [ ] 8.7 `../spec/worker.md` §Error Handling two-mode split — Phase 5.1 error convention (block failure → `Status==Error`, nil err; infra failure → non-nil err + Nack-no-requeue).  Phase 6.1 main loop enforces "publish then ack."
- [ ] 8.8 `../spec/worker.md` §Map Block Handling — `core.Execute` reads the expansion when `manifest.Kind == BlockKindMap` and returns it on `BlockInvocationResult.Expansion`.  The runner forwards it onto `WorkerResult.Expansion` verbatim.
- [ ] 8.9 `../spec/worker.md` §Communication — `broker/` package: two durable queues, persistent messages, prefetch 1, JSON bodies, ack-after-result, idempotent queue declaration.
- [ ] 8.10 `../spec/blocks.md` §4 execution env, §5 inputs, §6 outputs, §7 `invocation.yaml` — all covered by `core/block.go` + `core/executor.go`.
- [ ] 8.11 `../spec/pipeline.md` §5.4 resolution algorithm — `core.ResolveInputs` implements the four-step algorithm including the `typesCompatible` extensions for map→reduce chains.
- [ ] 8.12 `../spec/scheduler.md` — out of scope for the runner (runner is a worker, not a scheduler).  `core/scheduler.go` covers the scheduling side and is consumed by the CLI / server, not the worker.

## Phase 9 — Documentation

- [ ] 9.1 `runner/README.md`:
  - [ ] Overview: "Spade worker binary, consumes `spade.jobs`, publishes `spade.results`.  Delegates all execution primitives to `../core/`."
  - [ ] Build: `go build -o spade-worker ./cmd/spade-worker`.
  - [ ] Run: example with each flag and env var.
  - [ ] Test: `go test ./...` for unit tests; `go test -tags integration ./...` for broker + isolate integration tests (requires Docker for RabbitMQ, Ubuntu with `isolate` installed).
- [ ] 9.2 Package doc comments (`// Package broker provides ...`, etc.) on every exported package.
- [ ] 9.3 Exported-identifier doc comments — required by `staticcheck` (added via `go vet ./...` + `staticcheck ./...` in CI).

---

## Out of Scope (Explicit Non-Goals)

Documented so the plan's completion criteria are unambiguous:

- **The scheduler itself** (DAG traversal, dispatch, invocation-ID assignment, map/reduce fan-out).  Those live in `core/scheduler.go` and are driven by the server / CLI, not by the worker.
- **Block installation** (`spade install`).  CLI responsibility (`../spec/cli.md`).  The runner only reads the registry written by the installer.
- **Pipeline validation** (`spade check`).  CLI responsibility.  The runner trusts the assignments it receives.
- **Security screening of uploaded collections.**  Server-side responsibility at upload time (§worker.md).
- **Cache-key stability across hosts.**  `core.ComputeCacheKey` folds in `runtime.Version()` / GOOS / GOARCH, which means cache hits are local to a worker.  Cross-worker cache-sharing (if wanted) is an enhancement, not a requirement of this spec.

---

## Completion criteria

This plan is complete when:

1. Every phase above has its checklist boxes checked.
2. `go build ./...` in `runner/` succeeds with no warnings.
3. `go test ./...` passes for unit tests.
4. `go test -tags integration ./...` passes on a host with `isolate` installed and Docker available.
5. The `spade-worker` binary connects to a real RabbitMQ broker, consumes a `core.WorkerAssignment`, runs a fixture block end-to-end under `isolate`, and publishes a `core.WorkerResult` that the test harness can read back off `spade.results`.
