# Plugin Registry (`spade_registry`)

The cloud **Plugin Registry** from [`spec/registry.md`](../spec/registry.md): the
service that screens, builds, signs, stores, and serves block-collection
artifacts to workers, and the source of truth for which collection versions are
allowed in production.

It is a separate component from the per-worker **block index** (`core/registry.go`
+ `worker.md`), which is a local SQLite cache. This service is the centralized
authority that holds the artifacts themselves.

## Trust chain

```
source (git commit) → screen → build → sign → store → fetch → verify → execute
```

Build happens **after** screening, against the same commit the registry pulled,
so the artifact is provably the result of the build pipeline running on screened
source — the registry, not the developer, controls the artifact bytes.

## Architecture

Two binaries in one module:

| Binary | Role | DB access |
|---|---|---|
| `cmd/registryd` | **Control plane**: HTTP API, lifecycle state machine, audit log, ed25519 signing, `/pubkeys`, worker fetch, metadata mirror, **build dispatcher** | Yes (shared Postgres) |
| `cmd/builder` | **Build worker**, runs inside a per-language container: clone → screen → build → upload unsigned tarball to S3 staging → report over HTTP | **No** |

Key decisions (confirmed with the project owner; some refine the spec):

- **Ephemeral container per build.** The dispatcher in `registryd` watches the
  build queue and launches a fresh, language-appropriate container per build via
  the docker socket, then destroys it. The build worker never touches the
  database — it talks to the registry only over its authenticated HTTP API.
  (Refines `hosting.md` §5.2's "build runner polls Postgres".)
- **Control plane signs.** The builder uploads the *unsigned* tarball to an S3
  staging prefix and reports its content hash. `registryd` holds the ed25519
  private key, verifies the hash, signs, promotes the artifact, and writes the
  `.sig`. The private key never reaches the untrusted build container.
- **Screening is a pluggable no-op** today (`builder.NoopScreener`), to become an
  AI agent later.
- **One real reference builder (Go).** Python/R/Rust/TS builders are interface
  stubs that return "unimplemented".

```
publish ─▶ registryd ─▶ build queue ─▶ dispatcher ─▶ docker run builder-<lang>
                                                          │ clone@sha
                                                          │ screen (noop)
                                                          │ go build → tar.gz
                                                          │ PUT s3://staging/…
                                                          ▼
registryd: verify hash ─▶ ed25519 sign ─▶ s3://artifacts/… + .sig ─▶ state=available
                                                          │
worker ─▶ GET /artifacts/… ─▶ verify sig vs /pubkeys ─▶ verify hash ─▶ run
```

## Package layout

```
cmd/registryd      control-plane entrypoint
cmd/builder        build-worker entrypoint (runs in a container)
internal/
  config           env configuration (12-factor; reuses the repo's S3_* vars)
  store            GORM models + queries (Postgres prod, SQLite tests)
  state            lifecycle state machine + authorization (registry.md §3, §11)
  sign             ed25519 signing + trusted-key set + rotation (§6)
  blob             object storage: FSStore (tests/local) + S3Store (MinIO/Spaces)
  auth             developer (session table) / worker (service token) / builder (per-job)
  audit            append-only audit log (§7.3)
  mirror           web_ui `blocks` metadata mirror (§10)
  api              Echo v5 HTTP API (§11)
  dispatch         build dispatcher + Launcher (Docker / in-process)
  builder          clone, screen, build, package, upload, report
  wire             shared HTTP JSON request/response types
  testutil         Go-collection git fixture for end-to-end tests
  integration      full publish→build→sign→fetch→verify e2e test
```

## API surface (registry.md §11)

| Endpoint | Auth | Purpose |
|---|---|---|
| `POST /publish` | developer | Submit `(repo_url, sha, collection, version)` |
| `GET /collections` | public/worker | List collections |
| `GET /collections/:name/versions` | public/worker | List versions + states |
| `GET /collections/:name/:version/status` | developer | Screening/build status + logs |
| `POST /collections/:name/:version/state` | developer/operator | Deprecate / yank / recall |
| `GET /artifacts/:name/:version/:platform/:artifact` | worker | Download tarball or `.sig` |
| `GET /pubkeys` | public/worker | Trusted public key set |
| `GET /builds/:id` … `/screening` `/complete` `/fail` | builder (per-job) | Build worker callbacks |
| `GET /healthz` | public | Liveness |

The artifact segment is `<arch>` / `<arch>.tar.gz` (tarball) or `<arch>.sig`
(signature). Fetch is refused with `410 Gone` for `yanked`/`recalled` versions
so the worker invalidates its local index.

## Configuration (env)

`registryd`: `LISTEN_ADDR`, `DATABASE_URL` (empty → `SQLITE_PATH`), `S3_ENDPOINT`
/ `S3_REGION` / `S3_BUCKET` / `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY` /
`S3_USE_PATH_STYLE` (empty S3 → filesystem `BLOB_DIR`), `STAGING_PREFIX`,
`ARTIFACT_PREFIX`, `REQUIRE_APPROVAL`, `ADMIN_USER_IDS`, `BUILDER_IMAGES`
(`go=img:tag,…`), `BUILDER_DOCKER_ARGS`, `BUILD_TIMEOUT`, `SIGNING_KEY_SOURCE`
(`db`|`env` + `SIGNING_PUBLIC_KEY`/`SIGNING_PRIVATE_KEY`), `MIRROR_ENABLED`,
`REGISTRY_PUBLIC_URL`, `REGISTRY_INTERNAL_URL`.

`builder`: `REGISTRY_URL`, `BUILD_JOB_ID`, `BUILD_TOKEN`, `S3_*`, `STAGING_PREFIX`
(all injected by the dispatcher).

## Running

Standalone (SQLite + filesystem blob, no Docker — dispatcher idle without a
build image):

```sh
go run ./cmd/registryd   # serves :8090, generates a signing key on first run
curl localhost:8090/healthz
curl localhost:8090/pubkeys
```

Full local stack (Postgres + MinIO + ephemeral Docker builds):

```sh
docker compose up --build registry builder-go-image
```

The `registry` service mounts the docker socket and joins spawned build
containers to the compose network via `BUILDER_DOCKER_ARGS`. If your compose
project name differs from `psae_monorepo`, update that network name.

## Testing

```sh
go test ./...        # unit + integration (uses a real Go build + git fixture)
go vet ./...
```

The end-to-end test (`internal/integration`) drives publish → dispatch →
in-process build → sign → store → fetch → verify and the recall path entirely
offline (filesystem blob, SQLite, in-process launcher, real `go build`). It
skips if the `go`/`git` toolchains are unavailable.

## Signing & key rotation (registry.md §6)

Artifacts are signed with ed25519; workers carry only public keys. Rotation is
flag-day-free: `Keyset.AddKey` makes a new key active while the old key stays in
`/pubkeys` (existing signatures keep verifying), and `Keyset.RetireKey` removes
it once artifacts have been re-signed or aged out.

For local/dev the private key is stored in the DB. **Production must source the
key from a secret manager** (`SIGNING_KEY_SOURCE=env`, or extend `sign.Keyset`
to a KMS). This is the documented production path; KMS integration is out of
scope here.

## Known limitation: mirror schema

The metadata mirror writes into the web_ui-owned `blocks` table, whose current
columns are `id, name, label, version, manifest`. Collection/version/state
context is folded into the `manifest` jsonb. A coordinated web_ui drizzle
migration adding `collection` and `state` columns is the recommended follow-up;
until then `deprecated`/`yanked`/`recalled` versions are removed from the browse
mirror (the registry tables remain the source of truth and the mirror is
rebuildable).

## Out of scope (per registry.md §12)

Real screening pipeline (AI agent); real Python/R/Rust/TS builders; KMS-backed
keys; the scheduler/queue fleet-flush that pairs with recall; the local block
index schema; the CLI command surface.
