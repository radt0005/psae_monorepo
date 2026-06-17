# Plugin Registry — Implementation Plan

This plan takes `registry/` from an empty `go.mod` to a working, well-tested
Go implementation of the **Plugin Registry** specified in `../spec/registry.md`,
with the integration contracts in `../spec/worker.md` (fetch/recall),
`../spec/cli.md` (`spade publish`/`spade login`), `../spec/blocks.md`
(collection/manifest model), and `../spec/hosting.md` (topology).

## 0. Architecture summary (decisions)

These decisions were confirmed with the project owner and shape every phase.
They refine — and in two places deliberately deviate from — the spec.

- **Two binaries, one Go module (`spade_registry`):**
  - `cmd/registryd` — the **control plane** (App Platform service in
    `hosting.md` §3). Owns the API, lifecycle state machine, audit log,
    metadata mirror, **ed25519 signing**, the `/pubkeys` keyset, worker fetch,
    and the **build dispatcher**. **Shares the Postgres DB.**
  - `cmd/builder` — the **build worker**, runs *inside a per-language
    container*. Does screen → build → push artifact to S3 staging → report
    result. **Has no DB access**; talks to `registryd` only over its
    authenticated HTTP API.
- **Build dispatch = ephemeral container per build.** A dispatcher goroutine in
  `registryd` watches the build queue and launches a fresh, language-appropriate
  container per build via the Docker API, then destroys it. (Deviates from
  `hosting.md` §5.2 "build runner polls Postgres" — the build worker never
  touches the DB.) Dispatch is abstracted behind a `Launcher` interface so tests
  run without Docker.
- **Control plane signs, not the builder.** The builder uploads the *unsigned*
  tar.gz to an S3 *staging* prefix and reports its content hash; `registryd`
  holds the private key, verifies the hash, signs, promotes the artifact to the
  final key, writes the `.sig`, and sets state `available`. The private key
  never reaches the untrusted build container.
- **Screening is a pluggable no-op now** (becomes an AI agent later), behind a
  `Screener` interface.
- **Build depth: pluggable interfaces + one real Go reference builder.** Python,
  R, Rust, and TS builders are interface stubs returning "unimplemented".
- **Auth:** developers via Better Auth session token validated against the
  shared `session` table; workers via a hashed, rotatable service token;
  builders via a per-job short-lived token; operators via an admin allowlist.
- **Reuse `../core`** (via `replace core => ../core`) for `core.BlockManifest`,
  `core.DetectLanguage`, `core.CollectionLanguage`, and manifest scanning. Do
  not reimplement manifest parsing. Note `core/registry.go` is the *local
  SQLite block index*, unrelated to this cloud service.
- **Compose wiring is the final phase**, replacing the `seed-blocks` shim
  (kept as a fallback).

Trust chain (the ordering is the security argument; preserve it exactly):

```
source (git commit) → screen → build → sign → store → fetch → verify → execute
```

---

## Phase 1: Module plumbing

### 1.1 `go.mod` and dependencies
- [x] Rename module `registry` → `spade_registry` (unambiguous cross-module import path, matching `spade_server`).
- [x] Add `replace core => ../core` and `require core v0.0.0`.
- [x] Add direct dependencies (pin to the versions already used across sibling modules where they exist):
  - [x] `github.com/labstack/echo/v5` (HTTP; v5 per project owner).
  - [x] `gorm.io/gorm`, `gorm.io/driver/postgres`, `gorm.io/driver/sqlite` (Postgres in prod, SQLite in tests — same pattern as `server/`).
  - [x] `github.com/google/uuid`.
  - [x] `gopkg.in/yaml.v3` (parse `blocks/*.yaml` and collection manifests).
  - [x] `github.com/aws/aws-sdk-go-v2`, `.../config`, `.../credentials`, `.../service/s3` (S3-compatible blob store; works with MinIO + DO Spaces; uses the existing `S3_*` compose env vars).
  - [x] `github.com/docker/docker/client` (ephemeral build-container launcher).
  - [x] `github.com/stretchr/testify` (assertions, matches the repo).
  - [x] ed25519 + hashing come from the stdlib (`crypto/ed25519`, `crypto/sha256`); no dependency needed.
- [x] Run `go mod tidy`; keep it clean after every phase that adds imports.

### 1.2 Layout and build smoke
- [x] Create the package skeleton:
  ```
  registry/
    cmd/registryd/main.go        cmd/builder/main.go
    internal/
      config/      store/        state/       sign/
      blob/        auth/         audit/       mirror/
      api/         dispatch/     wire/        builder/
      collection/  testutil/
    testdata/
    Dockerfile.registryd
    Dockerfile.builder-go        Dockerfile.builder-python (stub)
  ```
- [x] Add `.gitignore` for built binaries (`registryd`, `builder`), `*.db`/`*.db-wal`/`*.db-shm` test artifacts, and a local `tmp/` workspace.
- [x] Confirm `go build ./...` succeeds with stub `main.go` files importing `core` and `echo`.

---

## Phase 2: Domain model and store (`internal/store`)

GORM models + queries. Postgres in production; SQLite for unit tests.

### 2.1 Models (the registry's own authoritative tables)
- [x] `Collection`: `id` (uuid), `name` (unique), `owner_user_id`, `language`, `description`, `created_at`, `updated_at`.
- [x] `Version`: `id`, `collection_id` (FK), `version` (semver string), `state` (enum, see Phase 5), `repo_url`, `commit_sha`, `submitted_by_user_id`, `error` (text, for `failed`), `created_at`, `updated_at`. Unique `(collection_id, version)`.
- [x] `Artifact`: `id`, `version_id` (FK), `platform`, `arch`, `content_hash` (sha256 hex), `artifact_key`, `sig_key`, `signing_key_id` (FK), `size_bytes`, `created_at`. Unique `(version_id, platform, arch)`. Key format: `<collection>/<version>/<platform>/<arch>.tar.gz` (+ `.sig`).
- [x] `BlockMeta`: `id`, `version_id` (FK), `block_id`, `name`, `kind`, `network` (bool), `description`, `entrypoint`, `inputs` (json), `outputs` (json). Populated from `blocks/*.yaml` at build completion; feeds the mirror.
- [x] `ScreeningResult`: `id`, `version_id` (FK), `screener_name`, `screener_version`, `passed` (bool), `details` (json/text), `created_at`.
- [x] `BuildJob` (the queue): `id`, `version_id` (FK), `language`, `state` (`queued`/`claimed`/`running`/`succeeded`/`failed`), `token_hash` (per-job builder token), `container_id`, `attempts`, `logs_key`, `claimed_at`, `created_at`, `updated_at`.
- [x] `SigningKey`: `id`, `public_key` (base64), `private_key` (base64; dev-only — see Phase 5 note), `active` (bool, exactly one), `listed` (bool, in `/pubkeys`), `created_at`, `retired_at` (nullable).
- [x] `ServiceToken`: `id`, `name` (worker id), `token_hash`, `scope` (`read`), `active` (bool), `rotated_from_id` (nullable), `last_used_at`, `created_at`.
- [x] `AuditEntry`: `id`, `event_type` (`publish`/`transition`/`fetch`/`screening`/`build`), `actor_id`, `actor_type` (`developer`/`worker`/`operator`/`system`), `collection`, `version`, `from_state`, `to_state`, `reason`, `detail` (json), `created_at`.

### 2.2 Store API and tests
- [x] `Open(dsn)` for Postgres and `OpenSQLite(path)`/`OpenSQLiteMem()` for tests; `AutoMigrate` all registry-owned models (do **not** auto-migrate the web_ui-owned `blocks`/`session` tables).
- [x] CRUD + query helpers: create collection/version, transition state (transactional), claim next queued build job (`SELECT ... FOR UPDATE SKIP LOCKED` on Postgres; serialized for SQLite), look up artifact by `(name, version, platform, arch)`, list collections/versions, write audit entries, manage signing keys and service tokens.
- [x] **Tests:** model round-trips, unique-constraint enforcement, atomic single-claim of a build job under concurrency, state-column persistence (SQLite in-memory).

---

## Phase 3: Configuration (`internal/config`)
- [x] Load config from env (12-factor; App Platform/compose friendly). registryd:
  - [x] `LISTEN_ADDR` (default `:8090`), `DATABASE_URL`.
  - [x] `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET` (artifacts), `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_USE_PATH_STYLE` (true for MinIO) — reuse the existing compose var names.
  - [x] `STAGING_PREFIX` (default `staging/`), `ARTIFACT_PREFIX` (default `artifacts/`).
  - [x] `REQUIRE_APPROVAL` (bool; gate `screened`→`building`).
  - [x] `ADMIN_USER_IDS` (comma list; operator authz for recall + token/key admin).
  - [x] `BUILDER_IMAGES` (lang→image map, e.g. `go=spade-builder-go:latest`).
  - [x] `DOCKER_HOST` (launcher; default local socket), `BUILD_TIMEOUT`.
  - [x] `SIGNING_KEY_SOURCE` (`db` default | `env`), optional `SIGNING_PRIVATE_KEY`/`SIGNING_PUBLIC_KEY`.
  - [x] `WEBUI_DSN`/mirror toggle (default: same DB as `DATABASE_URL`).
- [x] builder (separate, minimal): `REGISTRY_URL`, `BUILD_JOB_ID`, `BUILD_TOKEN`, plus the `S3_*` vars (write to staging only).
- [x] **Tests:** defaults, required-var errors, path-style flag parsing.

---

## Phase 4: Blob storage (`internal/blob`)
- [x] `Store` interface: `Put(ctx, key, r, size, contentType)`, `Get(ctx, key) (io.ReadCloser, error)`, `Exists`, `Delete`, `Copy(src,dst)`, `PresignGet(key, ttl)` (optional).
- [x] `FSStore` — filesystem-backed; used by unit/integration tests and trivial local runs.
- [x] `S3Store` — aws-sdk-go-v2 against MinIO/Spaces; honors path-style for MinIO.
- [x] **Tests:** FSStore round-trip, copy/delete/exists; S3Store behind a build tag against a local MinIO (skipped when unavailable).

---

## Phase 5: Lifecycle state machine (`internal/state`)
- [x] Define the `State` enum exactly per `registry.md` §3: `submitted`, `screening`, `screened`, `building`, `available`, `deprecated`, `yanked`, `recalled`, `failed`.
- [x] `AllowedTransitions` table encoding §3:
  - Happy path: `submitted`→`screening`→`screened`→`building`→`available`.
  - `available`→`deprecated`; `available`/`deprecated`→`yanked`; **any**→`recalled`; build/screen steps →`failed`.
  - `recalled` is terminal/irreversible.
- [x] `Authorize(actor, from, to)` per §11:
  - `deprecate`/`yank`: collection **owner** (or operator).
  - `recall`: **operator** (configurable — default operator-only).
  - System/internal transitions (`screening`→…→`available`, `→failed`): driven by the build pipeline, not external callers.
- [x] `Apply(...)` performs the DB transition + writes an audit entry + enqueues a mirror update, atomically.
- [x] **Tests:** every legal transition accepted; every illegal one rejected; owner vs. non-owner vs. operator authz; recall-from-any-state; recall irreversibility.

---

## Phase 6: Signing & key management (`internal/sign`)
- [x] `GenerateKeypair()`, `Sign(priv, data) []byte`, `Verify(pubs [][]byte, data, sig) bool` (verify against a **list** of trusted keys — `registry.md` §6.1).
- [x] `Keyset` loaded from `SigningKey` rows (or env): the single `active` key signs; all `listed` keys are served by `/pubkeys`.
- [x] Rotation helpers (flag-day-free per §6.1): `AddKey` (new active, old demoted to listed-not-active), `RetireKey` (drop from `/pubkeys`).
- [x] Sign over the artifact bytes (stream the staged tar.gz). Record `signing_key_id` on the `Artifact`.
- [x] **Note:** for local/dev the private key is stored in the DB; document that production must source it from a secret manager (`SIGNING_KEY_SOURCE=env` or KMS) — out of scope to implement KMS here.
- [x] **Tests:** sign/verify happy path; verify fails for wrong key; multi-key verify (old+new) accepts both; rotation sequence keeps old signatures valid until retired; tampered bytes fail.

---

## Phase 7: Authentication & authorization (`internal/auth`)
- [x] `DeveloperVerifier` interface; `SessionTableVerifier` impl: validate `Authorization: Bearer <token>` against the shared `session` table (`token` match, `expiresAt > now()`) → `userId`. (Pluggable so an HTTP-introspection impl can be swapped in later.)
- [x] `WorkerAuth`: hash the presented service token (sha256), match an `active` `ServiceToken`; update `last_used_at`. Rotation: multiple active tokens may coexist (`rotated_from_id`) so a worker mid-rotation never drops work (`registry.md` §7.2).
- [x] `BuilderAuth`: per-job token — hash and match `BuildJob.token_hash` for the job id in the request path; reject if the job is past `running`.
- [x] `OperatorAuth`: developer-authenticated **and** `userId ∈ ADMIN_USER_IDS`.
- [x] Echo middleware factories: `RequireDeveloper`, `RequireWorker`, `RequireBuilder(jobID)`, `RequireOperator`, plus `OptionalDeveloper` (CLI public-read fallback per `cli.md`).
- [x] **Tests:** valid/expired/missing session; valid/invalid/inactive worker token; correct/incorrect per-job builder token; admin vs. non-admin operator gate.

---

## Phase 8: Audit log (`internal/audit`)
- [x] `Logger` writing `AuditEntry` rows for: publish requests (developer, repo, sha, ts), state transitions (state, actor, ts, reason), and fetches (worker, collection, version, ts) — `registry.md` §7.3.
- [x] Helper hooked into `state.Apply`, the publish handler, and the fetch handler.
- [x] **Tests:** each event type recorded with the right fields; fetch logging on artifact download.

---

## Phase 9: Metadata mirror (`internal/mirror`)
- [x] `Mirror` interface (`UpsertVersion`, `RemoveVersion`) with a `PostgresMirror` impl and a `NoopMirror` (tests).
- [x] On `→available`: upsert one row per block into the web_ui-owned `blocks` table (`id`, `name`, `label`, `version`, `manifest` jsonb), sourced from `BlockMeta`. On `→recalled`/`→yanked` (no new installs): remove the rows; on `→deprecated`: remove from browse per §3.1 (hidden, but install/exec still work via registry).
- [x] Treat mirror writes as **best-effort** (log on failure, never fail the transition) — the registry tables remain the source of truth and the mirror is rebuildable (`registry.md` §10). Add a `RebuildMirror()` that replays from registry tables.
- [x] **Note/risk:** the current `blocks` schema lacks `collection`/`state` columns; record this limitation and propose a coordinated web_ui drizzle migration (`collection`, `state`) as a follow-up. The mirror writer targets only existing columns so it works against the seeded stack today.
- [x] **Tests:** upsert on available, removal on recall/yank/deprecate, rebuild from registry tables (NoopMirror + a SQLite stand-in for the blocks table).

---

## Phase 10: Control-plane HTTP API (`internal/api`, `cmd/registryd`)

Implement the surface in `registry.md` §11 plus the internal builder-facing
endpoints. Echo v4; JSON bodies; consistent error envelope.

### 10.1 Developer endpoints
- [x] `POST /publish` (developer): body `(repo_url, commit_sha, collection, version)`. Create the collection if new (owner = caller); reject a version that already exists in a non-`failed` state; create `Version{state: submitted}` + `BuildJob{state: queued}` with a fresh per-job token; write audit; return `{version_id, status_url}`. (Caller-side preconditions — clean tree, pushed HEAD — are enforced by `spade publish` per `cli.md`; the registry trusts only what is reachable at the SHA.)
- [x] `GET /collections/:name/:version/status` (developer/owner): current state, screening results, build logs reference, signing status — `cli.md` "track progress".
- [x] `POST /collections/:name/:version/state` (developer/operator): `{to_state, reason}` → `state.Authorize` + `state.Apply` (deprecate/yank by owner; recall by operator).

### 10.2 Public / worker read endpoints
- [x] `GET /collections` (public/worker): list collections (exclude `deprecated`-only from browse).
- [x] `GET /collections/:name/versions` (public/worker): versions + their states (worker recall-freshness check reads this — `worker.md` §Recall).
- [x] `GET /pubkeys` (public/worker): the listed trusted public keys.

### 10.3 Worker fetch endpoints (worker token)
- [x] `GET /artifacts/:name/:version/:platform/:arch`: enforce state — `available`/`deprecated` serve; `yanked`/`recalled`/non-`available` refuse (`410 Gone` with reason, so the worker invalidates per `worker.md`). Stream from blob store (or `302` to a presigned URL). Write a fetch audit entry.
- [x] `GET /artifacts/:name/:version/:platform/:arch.sig`: same gating; serve the `.sig`.

### 10.4 Builder-facing internal endpoints (per-job builder token)
- [x] `GET /builds/:id`: job detail (repo_url, sha, language, staging prefix, expected key) for the launched container.
- [x] `POST /builds/:id/screening`: `{screener_name, version, passed, details}` → persist `ScreeningResult`; on pass set `screened` (then `building` if `!REQUIRE_APPROVAL`); on fail set `failed`.
- [x] `POST /builds/:id/complete`: `{platform, arch, content_hash, staging_key, blocks:[...]}` → verify the staged object's hash, **sign**, copy staging→final artifact key, write `.sig`, create `Artifact` + `BlockMeta` rows, delete staging, set `available`, mirror upsert, audit. (Phase 6 + 9.)
- [x] `POST /builds/:id/fail`: `{reason, logs_key}` → set `failed`, persist logs ref, audit.
- [x] `POST /builds/:id/logs` (optional streaming): append build logs ref.
- [x] **Tests (httptest + SQLite + FSStore + NoopMirror):** publish creates version+job; status reflects state; state transitions honor authz; fetch gating for each state; builder endpoints drive the full happy path; complete rejects a hash mismatch (no signing, no promotion).

---

## Phase 11: Build dispatcher + container launcher (`internal/dispatch`)
- [x] `Launcher` interface: `Run(ctx, image, env, mounts) (containerID string, exitCode int, err error)`.
- [x] `DockerLauncher` — docker SDK: `docker run --rm` the language image with `BUILD_JOB_ID`, `BUILD_TOKEN`, `REGISTRY_URL`, and staging-scoped `S3_*` env; **no DB credentials**; capture container id + exit code.
- [x] `FakeLauncher` — for tests; runs the in-process `builder` flow (or canned results) so the e2e suite needs no Docker.
- [x] `Dispatcher` goroutine: poll `claim next queued BuildJob`; mark `claimed`→`running`; set version `screening`; resolve the image from `BUILDER_IMAGES[language]`; `Launcher.Run`; on container failure without a reported result, mark the job/version `failed`. Honor `BUILD_TIMEOUT`. Single concurrency at launch (per `hosting.md` §5.3), configurable.
- [x] **Tests:** dispatcher claims and launches exactly once; FakeLauncher drives a job to `available`; container nonzero-exit with no report → `failed`; timeout handling.

---

## Phase 12: Builder binary (`internal/builder`, `cmd/builder`)
Runs inside the per-language container. No DB; speaks only the registry API.

### 12.1 Registry client + flow
- [x] HTTP client authenticating with the per-job `BUILD_TOKEN`.
- [x] Flow: `GET /builds/:id` → `git clone` shallow at `commit_sha` into a fresh temp dir → run `Screener` → report screening → on pass run `Builder` → package tar.gz → compute sha256 → upload to `STAGING_PREFIX/<...>` → parse `blocks/*.yaml` (via `core`) → `POST /builds/:id/complete`. On any error: upload logs, `POST /builds/:id/fail`.
- [x] Use `core.DetectLanguage` to confirm the language matches the dispatched image; reuse `core.BlockManifest` for manifest parsing.

### 12.2 Screener and Builder interfaces
- [x] `Screener` interface: `Screen(ctx, srcDir) (ScreeningResult, error)`. `NoopScreener` returns pass with `screener_name=noop`. (Placeholder for the future AI agent.)
- [x] `Builder` interface: `Build(ctx, srcDir) (artifactDir string, err error)`.
- [x] `GoBuilder` (real): `go build` the collection into a single binary, assemble `binary + blocks/*.yaml` per `registry.md` §5, return the staged dir for packaging as `<collection>/<version>/<platform>/<arch>.tar.gz` (platform `linux`, arch `amd64`).
- [x] `PythonBuilder`, `RBuilder`, `RustBuilder`, `BunBuilder` — stubs returning a clear "unimplemented for <lang>" error.
- [x] Packager: deterministic tar.gz of the artifact dir; sha256 over the bytes.
- [x] **Tests:** NoopScreener passes; packager is deterministic + hash stable; GoBuilder builds a fixture Go collection (`testdata/`) and produces a runnable binary with subcommands; stub builders error cleanly; clone helper checks out the exact SHA.

---

## Phase 13: `registryd` wiring (`cmd/registryd`)
- [x] Compose config → store → blob → keyset → mirror → auth → api → dispatcher.
- [x] Bootstrap: ensure at least one signing key exists (generate on first run if none); optional bootstrap admin/service token via env for local dev.
- [x] Start the HTTP server and the dispatcher goroutine; graceful shutdown on SIGINT/SIGTERM (drain in-flight requests; let a running build finish or mark its job for redelivery). App Platform restart-tolerant per `hosting.md` §3.2.
- [x] Healthcheck endpoint (`GET /healthz`).

---

## Phase 14: Testing strategy (cross-cutting)
- [x] Unit tests accompany every package (enumerated above).
- [x] `internal/testutil`: SQLite-mem store factory, FSStore factory, fixtures, a fake registry server, and a fixture **Go collection git repo** (init a temp repo, commit, reference by SHA via `file://`).
- [x] **Integration (no Docker):** full `publish → dispatch(FakeLauncher) → builder flow → sign → store(FS) → fetch → verify` against the fixture Go collection, asserting the artifact verifies against `/pubkeys` and the hash matches metadata (mirrors the worker's verify steps in `worker.md` §Worker Installer).
- [x] **Recall path:** publish→available→recall→fetch returns `410`; `GET versions` reports `recalled` (the signal the worker acts on).
- [x] **Key rotation:** add a new key, re-verify old + new artifacts, retire old.
- [x] **Real-Docker e2e (build-tagged, skipped by default):** dispatcher with `DockerLauncher` builds the fixture Go collection in the real `spade-builder-go` image, end-to-end.
- [x] Ensure `go test ./...` is green; `go vet ./...` clean.

---

## Phase 15: Dockerfiles
- [x] `Dockerfile.registryd` — multi-stage build of `cmd/registryd` on a slim base (control plane needs no toolchains).
- [x] `Dockerfile.builder.go` — bundler-style image: Go toolchain + `git` + the `cmd/builder` binary; this is the per-language build container.
- [x] `Dockerfile.builder.python` / `.r` / `.rust` / `.ts` — stub images wrapping the stub builders (toolchain layers TBD), so the per-language model is wired even before the builders are real.

---

## Phase 16: docker-compose integration
- [x] Add a `registry` service (`build: ./registry`, `Dockerfile.registryd`) wired to `postgres` + `minio`, env from the existing `S3_*`/`DATABASE_URL` conventions; create the `spade-artifacts` bucket via the existing `createbuckets` pattern.
- [x] Give `registry` Docker socket access (or a sibling-container launch mechanism) so the dispatcher can start `spade-builder-go` per build; build the builder image(s) in compose.
- [x] Point the `worker` (and CLI default) at `REGISTRY_URL=http://registry:8080`; provision a worker service token; keep `seed-blocks` as a fallback path.
- [ ] Smoke test: `spade publish` a fixture Go collection → registry builds/signs/stores → `worker` fetches + verifies + executes a block end-to-end on the local stack. *(Not yet executed live: requires building the compose images and the worker's registry-fetch path. The equivalent chain — publish → build → sign → store → fetch → verify, plus recall — is covered offline by the `internal/integration` e2e test.)*

---

## Phase 17: Documentation
- [x] `registry/README.md`: architecture (the two binaries, trust chain, dispatch model), env/config reference, how to run locally (compose + standalone), the API surface, the signing/rotation model, and the known mirror-schema limitation + proposed follow-up.
- [x] Update the repo memory / cross-link `spec/registry.md` deviations (build worker off-DB, ephemeral container dispatch, control-plane signing).

---

## Out of scope (per `registry.md` §12, restated)
- The real screening pipeline (AI agent) — only the `Screener` interface + no-op here.
- Real Python/R/Rust/TS builders — interface stubs only.
- KMS/secret-manager-backed signing keys — DB/env sourcing only, with a documented production path.
- The work-queue/scheduler fleet-flush coordination on recall — the registry's contract ends at "no new invocations of a recalled version may execute".
- The local block index schema (`worker.md`) and the CLI command surface (`cli.md`).
```
