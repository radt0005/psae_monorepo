# Secrets Management

This document specifies how Spade pipelines obtain sensitive values -- database connection strings, data-source credentials, AI API keys -- at block execution time without those values ever appearing in pipeline YAML, logs, or any other insecure location.

The motivating use case: a user wants a block to query their own database and process the result with Spade.  The connection string must reach the block securely, must not be visible in the pipeline definition, and must not be readable by other users or other pipelines.

---

## 1. Headline Decisions

These decisions shape the rest of the document.  Rationale is in the sections that follow; this section records the choices.

1. **Worker-brokered, not block-brokered.**  Blocks never talk to the key-management service and never hold a credential for it.  The worker (in the cloud) or the CLI (in local runs) resolves the secrets a block declared, fetches the values, and injects them into the block.  The user trusts the worker with plaintext secrets in memory; the block is the untrusted party.

2. **Explicit secret mapping in the pipeline.**  A block author refers to a secret by a *logical name* (`get_secret("db")`).  The pipeline binds that logical name to one of the user's stored secrets.  Only the secret *name* ever appears in the pipeline -- never the value.

3. **Least privilege via scheduler-minted capability tokens.**  The scheduler mints a short-lived token scoped to exactly the secrets a given invocation declared.  The worker relays it to the KMS.  A compromised worker can read only the secrets of jobs currently flowing through it, not the whole secret store.

4. **Separate storage for local and cloud.**  Local runs read from the operating-system keychain; cloud runs read from the cloud KMS.  The two are independent stores and may hold different values under the same name (dev credentials vs. production credentials).

5. **A dedicated KMS service.**  The key-encryption key lives in a small standalone service whose only job is envelope crypto and authorization, not in the general-purpose server.  This isolates the master key from the system's largest attack surface.

---

## 2. Trust Model

Secrets cross a trust boundary at the sandbox edge.  Understanding that boundary is the foundation of the design.

### 2.1 The sandbox boundary (cloud)

In the cloud, blocks run under the `isolate` sandbox (see `worker.md`).  Relevant properties:

- **No network by default.**  Only blocks that declare `network: true` get network access.
- **Environment is allow-listed.**  Only environment variables the worker explicitly passes (`--env=NAME`) cross into the block.  Nothing leaks in otherwise.
- **Filesystem is confined** to the per-invocation working directory plus explicitly bound paths, and the block runs as a different uid.

This makes the worker a natural broker: it sits *outside* the sandbox with full authority, and it completely controls what crosses *into* the block.  A worker-brokered design runs with the grain of this boundary rather than against it.

### 2.2 The local edge case

`isolate` is Linux-only.  On macOS and Windows the CLI runs blocks as **plain subprocesses with the user's own authority** -- there is no filesystem confinement, no network restriction, no uid separation.

The consequence for secrets: on an unsandboxed local host, **no delivery mechanism is a security boundary against malicious block code**, because the block already runs as the user and could read the keychain, the environment, or any file the user can.  A block-brokered design (block calls the KMS itself) does not fix this -- a block running as the user can impersonate the user to the KMS directly.

Therefore the local goal is **hygiene, not confinement**: never persist plaintext where a backup, cloud-sync, or stray commit can capture it, and store secrets at rest encrypted (the OS keychain).  Both goals are met without a sandbox.

### 2.3 Why worker-brokered

Given the two environments above:

- In the **cloud**, where `isolate` confines the block and one worker may hold many users' secrets, least privilege matters.  Worker-brokering keeps any reusable KMS credential *out* of the sandbox and hands the block only its declared secrets.
- In **local** runs, brokering costs nothing over the alternative (neither approach confines a malicious block) and keeps the block-facing contract identical to the cloud.

The alternative -- the block calling the KMS directly -- was rejected.  It would couple secret access to `network: true`, require a KMS credential inside the sandbox that a malicious block could exfiltrate, and still need a local shim for offline runs.

---

## 3. Declaring Secrets

Secret usage is declared in two places, at two levels of abstraction.

### 3.1 In block code (logical name)

The block author refers to a secret by a logical role name through the runtime library:

```python
from spade import run, get_secret, TabularFile

def handler() -> TabularFile:
    dsn = get_secret("db")          # logical name, not a stored secret name
    # ... connect and query ...
```

The logical name (`"db"`) describes the block's need ("the database I query").  It is part of the block's contract, documented like a parameter.

### 3.2 In the pipeline (binding)

The pipeline block entry binds each logical name to one of the user's stored secrets, using a `secrets:` map alongside `args:`:

```yaml
- id: 019cf4bc-2222-7000-0000-000000000000
  name: db.query
  inputs: []
  args:
    query: "SELECT * FROM parcels WHERE county = 'Cumberland'"
  secrets:
    db: prod-postgres-dsn          # value is the *name* of a stored secret
```

The value (`prod-postgres-dsn`) is the **name** of a secret in the user's store, never the secret itself.  This is the only place secrets appear in a pipeline, and only by reference.

This is an addition to the block-entry schema in `pipeline.md` §4 (`secrets` is optional; absent means the block declares no secrets).

### 3.3 Name resolution and portability

The pipeline references a secret by name.  Which store backs that name depends on where the pipeline runs:

- **Local run:** the name resolves against the OS keychain.
- **Cloud run:** the name resolves against the cloud KMS.

The two stores are independent (see §1.4).  The same name may hold a dev value locally and a production value in the cloud, so one pipeline file runs correctly in both places without edits.  Storing separate values is deliberate: local and cloud database credentials, URLs, and API keys are commonly different.

---

## 4. Delivery to the Block

Secrets reach the block as a single injected environment variable, `SPADE_SECRETS`, containing a JSON object keyed by logical name:

```
SPADE_SECRETS={"db":"postgresql://user:pass@host:5432/db"}
```

- **Cloud:** the worker injects it via `isolate --env=SPADE_SECRETS=<json>`.  The value is set directly into the sandbox environment and is **never written to disk**.
- **Local:** the CLI sets it on the block subprocess's environment.  Also never written to disk.

The runtime library reads `SPADE_SECRETS` at startup, exposes each value through `get_secret(name)`, and scrubs the variable from the environment so it is not inherited by any grandchild process the block spawns.

Delivering values in the process environment rather than a file is what keeps plaintext off the filesystem on every platform -- this is the mechanism that resolves the local-hygiene requirement of §2.2.  (An inherited file descriptor or anonymous pipe is a marginally more hardened alternative than an environment variable, since environment values can surface in crash dumps and `/proc/<pid>/environ`; it is deferred because it is substantially fiddlier across the five runtime languages and three operating systems.  The env-blob is the specified mechanism; fd-passing is a later hardening option.)

### 4.1 Library API

Every runtime library (Python, R, TypeScript, Go, Rust) exposes the same function.  Because delivery is always the pre-populated `SPADE_SECRETS` blob, each library does exactly one thing -- parse that variable and serve values.  **No library contains keychain, KMS, or network code.**  All storage and fetch logic lives in the Go worker and Go CLI.

- `get_secret(name) -> string` returns the secret bound to the logical `name`.
- If the block requests a name that was not injected (not declared in the pipeline, or resolution failed), `get_secret` raises/returns an error and the block fails.  A declared-but-unresolvable secret is a real error, not a silent empty value.

---

## 5. The Key-Management Service (KMS)

The KMS is a small dedicated service, deployed on App Platform in the same VPC as the rest of the control plane (see `hosting.md`).  It is deliberately separate from the general-purpose server so the key-encryption key (KEK) is isolated from the system's largest attack surface.

### 5.1 Storage and encryption

Secrets are stored with **envelope encryption**:

- Each secret value is encrypted with a per-secret data-encryption key (DEK).
- DEKs are wrapped by a master **key-encryption key (KEK)** held only by the KMS service (from DO's secret store / service environment, never in the database).
- Ciphertext is stored in the managed PostgreSQL cluster (`hosting.md` §6.1) in a `secrets` table keyed by owner identity and secret name.

A database compromise alone yields only ciphertext; decryption requires the KEK, which lives only in the KMS process.

### 5.2 Identity and ownership

Secrets are owned by a **user identity** provided by Better Auth (see `hosting.md` §7), with optional sharing to an organization.  The management endpoints authenticate the caller as that user; the resolve endpoint authorizes against the owner encoded in the capability token (see §6).

### 5.3 API surface

Sketch of the HTTP/JSON API.  Path versioning and error formats are an implementation concern.

| Endpoint                        | Caller             | Purpose                                                       |
| ------------------------------- | ------------------ | ------------------------------------------------------------ |
| `PUT /secrets/<name>`           | User (web UI/CLI)  | Create or update a secret value                              |
| `GET /secrets`                  | User (web UI/CLI)  | List secret **names** (never values)                        |
| `DELETE /secrets/<name>`        | User (web UI/CLI)  | Delete a secret                                              |
| `POST /resolve`                 | Worker             | Exchange a capability token for the scoped secret **values** |

The management endpoints never return secret values -- only names and metadata.  `POST /resolve` is the only endpoint that returns plaintext, and only for the names a valid capability token authorizes.

### 5.4 Audit log

The KMS records, mirroring the registry's audit model (`registry.md` §7.3):

- Secret writes and deletes (user, name, timestamp)
- Resolves (owner, invocation ID, secret names, worker identity, timestamp) -- for incident response

---

## 6. Capability Tokens

The worker's authority to read secrets is granted per-invocation, not standing.  This is what bounds the blast radius of a compromised worker.

### 6.1 Minting

When the scheduler dispatches a job whose block declares secrets, it mints a **capability token** and includes it in the `spade.jobs` message (see `worker.md`).  The token asserts:

```
{ user_id, invocation_id, secret_names: [...], exp }
```

`secret_names` is exactly the set the block declared in its `secrets:` map -- no more.  The token is **signed with the scheduler's private key** using ed25519, the same signature scheme and rotation model already used for registry artifacts (`registry.md` §6).  The KMS verifies it with the scheduler's public key.  Verification is stateless: the scheduler and KMS make no runtime call to each other, they only share a trusted public key.

### 6.2 Redemption

The worker calls `POST /resolve` with the token and the specific names it needs.  The KMS:

1. Verifies the signature against the scheduler's trusted public key(s).
2. Checks the token has not expired.
3. Checks the requested names are a subset of the token's `secret_names`.
4. Checks the token's `user_id` owns (or is shared) each secret.
5. Decrypts and returns the values.

The worker re-keys the returned values from stored-secret names to the block's logical names (using the pipeline's `secrets:` map, which is in the job message) and builds the `SPADE_SECRETS` blob.

### 6.3 Not single-use

The token is **time-boxed and invocation-scoped, but not single-use**.  Job delivery is at-least-once (`worker.md`): a worker can fetch secrets, crash before acking, and the job is redelivered to another worker that must resolve the same secrets again for the same `invocation_id`.  Burning the token on first use would break redelivery.  Short expiry plus invocation scoping bounds replay without breaking the retry semantics.  (This mirrors the scheduler's idempotent-results handling in `worker.md`.)

---

## 7. End-to-End Flow (cloud)

1. The user stores `prod-postgres-dsn` via the web UI.  The KMS envelope-encrypts it and writes ciphertext to PostgreSQL.
2. A pipeline block declares `secrets: { db: prod-postgres-dsn }`.
3. The scheduler dispatches the invocation and mints a capability token scoped to `[prod-postgres-dsn]` for this `invocation_id` and owner, placing it in the `spade.jobs` message.
4. The worker receives the job, calls `POST /resolve` with the token, and receives `{ prod-postgres-dsn: "postgresql://..." }`.
5. The worker re-keys to the logical name and injects `SPADE_SECRETS={"db":"postgresql://..."}` via `isolate --env`.
6. The block calls `get_secret("db")`, connects, and runs.  If the block also needs to reach the database over the network, its manifest must declare `network: true` (independently of secret access).

Local runs follow the same shape with the CLI substituted for the worker+scheduler: the CLI reads the named secrets from the OS keychain and sets `SPADE_SECRETS` on the block subprocess.  No capability token is involved locally.

---

## 8. Local Secret Management

Local secrets live in the operating-system keychain (macOS Keychain, Windows Credential Manager, libsecret on Linux), accessed through a Go keychain library imported only in the CLI.

- `spade secret set <name>` prompts for a value and stores it in the keychain.
- `spade secret list` lists names.
- `spade secret rm <name>` deletes.
- `spade secret set --remote <name>` targets the cloud KMS instead of the keychain, for convenience from the CLI.

Local and cloud namespaces are independent (§3.3).  There is no automatic sync of plaintext between them.

---

## 9. Key Rotation

Two independent keys rotate on their own schedules, both using the list-of-trusted-keys pattern from `registry.md` §6.1 to avoid flag-day cutovers:

- **KEK rotation** re-wraps DEKs under a new master key.  Old and new KEKs are both accepted during the window; the KMS re-wraps lazily or in a background pass, then retires the old KEK.
- **Scheduler signing-key rotation** adds the new public key to the KMS's trusted set before the scheduler switches to signing with the new key; the old key is retired after outstanding tokens have expired.

---

## 10. Out of Scope

- The screening of block source for secret misuse (e.g. a block that reads a secret and exfiltrates it) -- a registry screening concern (`registry.md` §4).  Any block with a legitimate secret and `network: true` can exfiltrate it; this is inherent and is addressed by screening and by surfacing `network: true` in the UI, not by the KMS.
- Network egress policy for blocks (allow-listing destination hosts) -- a future worker/sandbox concern.
- The detailed Better Auth deployment (`hosting.md` §7).
- Organization/team sharing semantics beyond the ownership model sketched in §5.2.
- Secret value versioning and history.
