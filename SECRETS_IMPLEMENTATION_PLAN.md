# Secrets Management — Implementation Plan

This plan implements the secrets design in [`spec/secrets.md`](spec/secrets.md):
a worker-brokered path that delivers user secrets (database connection strings,
API keys) into a sandboxed block without them ever appearing in pipeline YAML,
logs, or on disk.

The block-facing contract is one function, `get_secret("name")`, backed by an
injected `SPADE_SECRETS` environment blob.  All storage and fetch logic lives in
Go (worker + CLI); the language libraries only parse the blob.  Cloud secrets
live in a new **KMS service**; local secrets live in the OS keychain.

```
set secret ─► [cloud KMS | local keychain]
                     │
run ─► scheduler mints scoped token ─► worker /resolve ─► SPADE_SECRETS ─► block.get_secret()
       (cloud)                          (cloud)            (both)
run ─► CLI reads keychain ────────────────────────────────► SPADE_SECRETS ─► block.get_secret()
       (local)
```

---

## 0. Decisions (settled in `spec/secrets.md`)

- **Model B, worker-brokered.** Blocks never talk to the KMS. The worker (cloud)
  or CLI (local) resolves declared secrets and injects them. Plaintext-in-memory
  on the worker is accepted.
- **Delivery = `SPADE_SECRETS` env blob**, JSON `{logical_name: value}`, never
  written to disk. Cloud: `isolate --env=SPADE_SECRETS=<json>`. Local: set on the
  block subprocess env.
- **Explicit mapping in the pipeline.** `PipelineBlock.Secrets` maps the block's
  *logical* name → the user's *stored* secret name. Only names in YAML, never
  values.
- **Scheduler-minted, ed25519-signed capability token**, scoped to the block's
  declared stored-secret names, per `invocation_id`, time-boxed, **not
  single-use** (redelivery-safe).
- **Separate namespaces**: local (keychain) and cloud (KMS) are independent.
- **Dedicated KMS service**; the KEK lives only there.

---

## 1. Order of Work

Local delivery lands first (no cloud dependency), then the cloud path:

| Phase | Deliverable | Value |
| --- | --- | --- |
| 1 | Pipeline schema + core delivery plumbing | `SPADE_SECRETS` reaches the sandbox |
| 2 | `get_secret` in all 5 language libs | blocks can read secrets |
| 3 | CLI keychain management + local-run injection | **local end-to-end works, no cloud** |
| 4 | KMS service (crypto, storage, endpoints, audit) | cloud storage exists |
| 5 | Capability tokens (scheduler mint + Job field + KMS verify) | scoped authz |
| 6 | Worker resolve + inject | **cloud end-to-end works** |
| 7 | Wiring, rotation, docs | production-ready |

---

## 2. Phase 1 — Pipeline schema + core delivery

### 2.1 `PipelineBlock.Secrets`
`core/types.go:136` — add the mapping field:
```go
// Secrets maps a block's logical secret name (get_secret argument) to the
// user's stored secret name. Values are names, never secret values. See
// spec/secrets.md §3.2.
Secrets map[string]string `yaml:"secrets,omitempty"`
```
`Secrets` travels inside `Job.Pipeline`, so no separate Job field is needed for
the *map* (the capability *token* is added in Phase 5).

### 2.2 Thread resolved secrets into `core.Execute`
Resolved secrets are `map[string]string` (logical → **value**). They are
sensitive and must **not** be persisted on `BlockInvocation` or logged. Pass them
as an explicit argument, not a serialized field.

- `core/executor.go:49` — change `Execute(...)` to accept `secrets map[string]string`
  (or a small `ExecOptions` struct to avoid future signature churn).
- `runner/worker/worker.go:51-73` — update the `Executor` interface and
  `coreExecutor.Execute` to match.
- `cli/cmd/run.go:237` — update the call site.
- Fix the executor/worker tests that call these.

### 2.3 Inject `SPADE_SECRETS` into the sandbox
`core/executor.go:221` (the `--env` allow-list block in `RunBlockSubprocess`) —
when `len(secrets) > 0`, JSON-marshal the map and append:
```go
"--env=SPADE_SECRETS=" + string(jsonBlob)
```
`isolate --env=VAR=VALUE` sets the value explicitly; the arg is passed via
`exec.Command` (not a shell), so JSON quoting is safe. **The value is never
written to any file** — this is the property that keeps plaintext off disk on
every platform (`spec/secrets.md` §4). Confirm no code path logs the args/env of
the isolate command.

### 2.4 Test
Extend `core/executor_test.go`: a block that echoes `SPADE_SECRETS` sees the
injected value; assert no `secrets`/`params.yaml` file on disk contains it.

---

## 3. Phase 2 — `get_secret` in the libraries

Every lib does the same thing: read `SPADE_SECRETS` (JSON object) once at
startup, expose `get_secret(name)`, and **scrub the variable** from the
environment so grandchild processes don't inherit it. Missing name ⇒ error
(fail the block); a declared-but-unresolved secret is a real error, not empty.

| Lang | File | Export |
| --- | --- | --- |
| Python | `libs/python/src/spade/run.py` + `__init__.py` | `get_secret` |
| R | `libs/R/R/run.R` + `NAMESPACE` | `get_secret` |
| TypeScript | `libs/typescript/src/run.ts` + `index.ts` | `getSecret` |
| Go | `libs/go/run.go` | `GetSecret` |
| Rust | `libs/rust/src/run.rs` + `lib.rs` | `get_secret` |

Parse lazily and cache: read the env var on first `get_secret` call (or in
`run()` bootstrap), unset it immediately after reading. Add a unit test per lib:
set `SPADE_SECRETS={"db":"x"}`, assert `get_secret("db") == "x"`, assert missing
key errors, assert the env var is unset after first read.

---

## 4. Phase 3 — CLI keychain + local-run injection

### 4.1 Keychain wrapper
New `cli/internal/secretstore/` wrapping the Go keychain library
(`zalando/go-keyring` or `99designs/keyring`; the latter has a file backend for
headless/CI Linux where libsecret is absent). Service name e.g. `spade`, account
= secret name. Functions: `Set(name, value)`, `Get(name) (string, error)`,
`List()`, `Delete(name)`.

### 4.2 `spade secret` command group
New `cli/cmd/secret.go`:
- `spade secret set <name>` — prompt without echo (`golang.org/x/term`), store to
  keychain.
- `spade secret list` — names only.
- `spade secret rm <name>`.
- `spade secret set --remote <name>` — HTTP `PUT` to the KMS (Phase 4); requires
  `spade login` session. Deferred until Phase 4 exists.

### 4.3 Local resolution at run
`cli/cmd/run.go` (near :237): before `core.Execute` for each invocation, read the
`PipelineBlock.Secrets` map, look up each **stored** name in the keychain, build
the `{logical: value}` map, pass it into `Execute`. **No capability token
locally.** A referenced secret missing from the keychain ⇒ clear error naming the
secret and suggesting `spade secret set`.

**After this phase, secrets work end-to-end for `spade run` with zero cloud
infrastructure.**

---

## 5. Phase 4 — The KMS service

New top-level Go module `kms/` with `kms/cmd/spade-kms/` (mirrors the
`server/`/`registry/` layout). Deployed as its own App Platform service
(`hosting.md` §2); the KEK lives only in this process.

### 5.1 Storage (managed Postgres)
```
secrets(
  id, owner_id, name,
  ciphertext, value_nonce,      -- AES-256-GCM of the value under the DEK
  wrapped_dek, dek_nonce, kek_id,-- DEK wrapped by the KEK (kek_id selects which)
  created_at, updated_at,
  unique(owner_id, name)
)
secret_audit(id, owner_id, invocation_id, secret_names[], actor, action, at)
```

### 5.2 Envelope encryption
- Per secret: generate a random 256-bit DEK, AES-GCM encrypt the value, then
  AES-GCM wrap the DEK under the **KEK**.
- KEK(s) provided via env as a keyed set (`kid → base64 key`) to allow rotation;
  `kek_id` records which wrapped each DEK.
- Package `kms/internal/envelope` with a round-trip unit test.

### 5.3 Endpoints (`spec/secrets.md` §5.3)
| Endpoint | Auth | Purpose |
| --- | --- | --- |
| `PUT /secrets/<name>` | user session | create/update (encrypt + store) |
| `GET /secrets` | user session | list **names** only |
| `DELETE /secrets/<name>` | user session | delete |
| `POST /resolve` | capability token | return scoped **values** |

Management-endpoint auth: accept `X-Spade-User-Id` for now (matching
`server/api/server.go:134`), with Better Auth integration deferred (`hosting.md`
§7). `GET`/`PUT`/`DELETE` never return values; only `/resolve` returns plaintext.

### 5.4 Audit
Write a `secret_audit` row on every write, delete, and resolve (owner,
invocation, names, timestamp) — `spec/secrets.md` §5.4.

---

## 6. Phase 5 — Capability tokens

### 6.1 Token format & signing
Reuse the ed25519 machinery the registry already uses for artifact signing
(`registry/internal/...`). Token claims:
```json
{ "user_id": "...", "invocation_id": "...", "secret_names": ["prod-postgres-dsn"], "exp": 1720000000 }
```
Signed with the **scheduler's** private key. Put the signer in a shared package
(e.g. `core/captoken` or `kms/token`) usable by both scheduler (sign) and KMS
(verify).

### 6.2 Mint at dispatch
`server/engine/engine.go:472` (`BuildJobForInvocation`): if the invocation's
`PipelineBlock.Secrets` is non-empty, mint a token scoped to exactly those
**stored** names (the map values), for this `invocation_id` and the pipeline
**owner**. Attach it to the Job.
- The pipeline owner is recorded at submission (`server/api/server.go:134`,
  `X-Spade-User-Id`). **Dependency:** the engine must have the owner available at
  dispatch; wire it through if it isn't already.
- `runner/types.go` — add `CapabilityToken string \`json:"capability_token,omitempty\"\`` to `Job`.

### 6.3 Verify at resolve
KMS `/resolve`: verify signature against the scheduler's public key(s) (a **list**
for rotation, `registry.md` §6.1); check `exp`; check requested names ⊆
`secret_names`; check `user_id` owns each; decrypt; return values. **Not
single-use** — redelivery may resolve the same `invocation_id` twice
(`worker.md`). Unit-test: valid, expired, wrong key, out-of-scope name,
non-owner.

---

## 7. Phase 6 — Worker resolve + inject

`runner/worker/worker.go:157` (`Run`), before calling the executor:
1. Find this invocation's `PipelineBlock` in `job.Pipeline`; read its `Secrets`
   map. If empty, skip.
2. `POST /resolve` to `KMS_URL` with `job.CapabilityToken` and the stored names
   (the map values). New `runner/kmsclient/` HTTP client.
3. Receive `{stored_name: value}`; **re-key to logical names** using the
   `Secrets` map → `{logical: value}`.
4. Pass the `{logical: value}` map into the `Executor.Execute` call (Phase 2.2).

Resolve failure ⇒ worker-side failure (do **not** ack; job redelivers), matching
`worker.md` error handling. Never log the values.

**After this phase, secrets work end-to-end in the cloud.**

---

## 8. Phase 7 — Wiring, rotation, docs

- **`docker-compose.yml`**: add the `spade-kms` service; env for the KEK set,
  the scheduler signing keypair (private → scheduler, public → KMS), and
  `KMS_URL` for the worker.
- **Key rotation** (`spec/secrets.md` §9): KEK rotation re-wraps DEKs (lazy or
  background pass, keyed by `kek_id`); scheduler signing-key rotation adds the new
  public key to the KMS trusted set before the scheduler switches. Both use the
  trusted-list pattern.
- **Docs**: `documentation/content/` — `spade secret` commands and the
  `get_secret` API; `cli.md` command surface.
- **Hosting**: already reflected (`hosting.md` §2/§6.1/§9).

---

## 9. Cross-cutting requirements

- **Never log secret values.** Audit the worker, executor, and KMS log lines for
  args/env dumps. Add a test that greps captured logs for a sentinel value.
- **Signature/verify parity.** Scheduler and KMS must agree on the exact signed
  byte encoding — share one package, don't re-implement.
- **Env scrubbing.** Libs unset `SPADE_SECRETS` after reading so grandchildren
  don't inherit it (§3).
- **Headless keychain.** Ensure a file-backend fallback so `spade run` works in
  CI/headless Linux without libsecret.

---

## 10. Out of scope (this plan)

- Better Auth integration for the management endpoints (deferred; `X-Spade-User-Id`
  stand-in).
- Organization/team secret sharing beyond single-owner.
- Secret value versioning/history.
- fd/pipe delivery hardening (env blob is the specified mechanism; fd-passing is a
  later option, `spec/secrets.md` §4).
