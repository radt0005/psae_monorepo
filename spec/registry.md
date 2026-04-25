# Plugin Registry

The Plugin Registry is the cloud service that holds screened, built, and signed block collection artifacts and serves them to workers.  It is the source of truth for what collections are available in production and the foundation of the trust chain that lets workers safely execute third-party code.

Developers publish collections by pointing the registry at a git commit.  The registry screens the source, builds the artifact in the same base image used by workers, signs the result, and stores it.  Workers fetch signed tarballs at execution time, verify the signature against a known public key, unpack the artifact into the local block index, and run.

The registry is a separate component from the local **block index** described in `worker.md`.  The block index is a per-worker SQLite cache of what is installed on that worker; the registry is a centralized service that holds the artifacts themselves and decides which collection versions are allowed in production.

---

## 1. Trust Chain

The trust chain is ordered, and the order matters:

```
source (git commit) → screen → build → sign → store → fetch → verify → execute
```

Build **must** happen after screening.  If a developer could screen one source tree and upload a different binary, the screening signal would be meaningless.  By having the registry pull, screen, and build from the same commit SHA, the artifact is provably the result of running the build pipeline against the screened source.

This is the central security argument for git-based ingest: the registry, not the developer, controls the artifact bytes.

---

## 2. Ingest Model

The registry ingests collections from git, not as uploaded tarballs.

To publish a collection, the developer:

1. Pushes the collection to a git remote (GitHub, internal Gitea, etc.)
2. Runs `spade publish` from the collection's working directory

`spade publish`:

1. Runs `spade check` against the local working tree
2. Verifies the working tree is clean and the current HEAD is reachable on the configured remote
3. Submits `(repo_url, commit_sha, collection_name, version)` to the registry as a publish request

The registry then:

1. Shallow-clones the repo at the requested SHA into an ephemeral build environment
2. Runs the screening pipeline against the source
3. If screening passes (and any required human approval is granted), runs the language-specific build
4. Signs the resulting artifact with the registry's private key
5. Stores the artifact and metadata, transitioning the collection version to `available`

For private repos, the registry authenticates with a deploy key or GitHub App credential associated with the developer's account.  The credential needs **read access only** -- the registry never pushes back to the source repo.

---

## 3. Lifecycle States

A collection version moves through several states.  The registry persists the state and exposes it to clients.

| State         | Meaning                                                                                  |
| ------------- | ---------------------------------------------------------------------------------------- |
| `submitted`   | Publish request received, waiting in the screening queue                                  |
| `screening`   | Screening pipeline is running                                                             |
| `screened`    | Screening passed; awaiting human approval (if required) or automatic build                |
| `building`    | Build pipeline is running                                                                 |
| `available`   | Build, sign, and store complete; workers may install and execute                          |
| `deprecated`  | Hidden from browse/discovery; install and execution still work                            |
| `yanked`      | Blocks new installs from the registry; existing installs continue to work                 |
| `recalled`    | Refuses to execute; workers invalidate local block index entry on dispatch                |
| `failed`      | Screening, build, or sign step failed; surfaced to developer with logs                    |

Transitions:

- Happy path: `submitted` → `screening` → `screened` → `building` → `available`
- Soft hide: `available` → `deprecated`
- Block installs: `available` or `deprecated` → `yanked`
- Security recall: any state → `recalled`

Recall is irreversible at the version level.  To un-recall a collection, the developer publishes a new version with the fix and recalls all affected prior versions.

### 3.1 Three deactivation states

The three "off" states are intentionally distinct:

- **Deprecated** is a soft signal: hide it from the browse view in the web UI but do not interrupt anything that already uses it.  Use this when a successor version exists and you want to nudge users toward it.
- **Yanked** blocks new installs but does not interfere with workers that already have the artifact unpacked.  Use this when a version should not gain new users but is otherwise safe to keep running.
- **Recalled** is the hard switch.  It refuses execution at dispatch time, invalidates the local block index entry, and removes the unpacked artifact from disk.  Use this when continued execution is unsafe -- a vulnerability, malicious code, or a correctness defect serious enough to halt pipelines.

---

## 4. Screening

Screening is a **policy hook**, not a fixed pipeline.  The registry runs configured screeners against the cloned source and records the result.  Specific screener implementations (static analysis, license scanning, secret detection, manual review) are out of scope for this spec and may vary per deployment.

The registry must:

- Support pluggable screeners, configured per deployment
- Record structured results (pass/fail, details, timestamp, screener version)
- Allow re-screening a previously-screened version (e.g. after a screener update)
- Optionally require human approval before transitioning from `screened` to `building`, configurable per collection or per organization

Human approval is a policy choice, not a hard requirement.  A closed/internal deployment may run fully automated; a public-facing deployment may require approval for all new collections or for major version bumps.

---

## 5. Build

The registry builds collections in the same base image as the worker, plus toolchains.  This guarantees the build environment matches the runtime environment exactly -- same glibc, same system libraries, same architecture.

The bundler image is the worker base image plus the language toolchains and headers needed to compile native dependencies (`cargo`, `go`, `bun`, `gcc`, `clang`, GDAL development headers, R headers, etc.).  The worker image itself contains only the language **runtimes** -- `python3`, `R`, the GDAL system library, and the worker binary.

Build outputs by language:

- **Rust / Go / TypeScript (Bun):** a single binary plus the `blocks/*.yaml` manifests, packaged as a `.tar.gz`.
- **Python:** the source tree, the `uv` lockfile, and a populated dependency cache for the lockfile, packaged as a `.tar.gz`.  The worker unpacks and runs; no further `uv sync` is required.
- **R:** the source tree, the `renv.lock` file, and a populated `renv` library, packaged as a `.tar.gz`.

For each supported platform/architecture pair, a separate artifact is produced.  The current target is Debian Linux on `amd64`.  The artifact key is:

```
<collection>/<version>/<platform>/<arch>.tar.gz
```

Adding a new platform later (for example, `arm64`) does not require any structural change to the storage layout.

---

## 6. Signing

Each artifact is signed with the registry's private key using **ed25519** (an asymmetric digital signature).  The signature is stored alongside the artifact:

```
<collection>/<version>/<platform>/<arch>.tar.gz
<collection>/<version>/<platform>/<arch>.tar.gz.sig
```

Asymmetric signatures are required, not HMAC.  HMAC would force every worker to hold the secret needed to forge signatures; ed25519 lets the registry hold the private key while workers carry only the public key.  A worker compromise cannot produce forged artifacts for other workers.

### 6.1 Key Rotation

Workers must accept a **list** of trusted public keys, not a single key.  During a rotation:

1. The registry generates a new keypair and adds the new public key to the trusted-keys endpoint
2. Workers refresh their trusted-keys list and accept signatures from either the old or new key
3. The registry switches to signing new artifacts with the new key
4. After all artifacts signed by the old key have been resigned or aged out, the old key is removed from the trusted-keys list

This avoids a flag day during rotation.

---

## 7. Authentication

Two distinct identity types interact with the registry.

### 7.1 Developers

Developers authenticate via **Better Auth** (the system's user identity provider, replacing PocketBase).  The CLI obtains a session via `spade login`, which performs an OAuth-style flow against the Better Auth endpoint and stores the resulting credentials locally for use by `spade publish` and other authenticated commands.

Any developer with verified contact details may publish a collection for screening.  Whether a publish reaches `available` depends on screening (and optional human approval) policy, not on identity.

Developers may transition state only on collections they own.  Recall and other operator-level transitions may be restricted to administrators.

### 7.2 Workers

Workers authenticate to the registry's fetch endpoints with a **rotated service token**, scoped to read-only access.  The token is provisioned at worker setup and rotated on a schedule.  Workers must handle a token rotation without dropping work in flight.

### 7.3 Audit log

The registry maintains an audit log of:

- Publish requests (developer identity, repo, SHA, timestamp)
- State transitions (state, actor, timestamp, reason)
- Fetches (worker identity, collection, version, timestamp) -- for incident response

---

## 8. Worker Fetch

When a worker is asked to execute a block from a collection it does not have installed (or whose version it does not have installed), it:

1. Queries the registry's fetch endpoint for `<collection>/<version>/<platform>/<arch>`
2. Downloads the tarball and the signature
3. Verifies the signature against the trusted public keys
4. Verifies the artifact's content hash matches the value recorded in the registry metadata
5. Unpacks the tarball into `~/.spade/blocks/<collection>/<version>/`
6. Updates the local block index (see `worker.md`)

The fetch path uses **no toolchains** -- no `cargo`, `uv`, or `Rscript` invocations on the worker.  The worker base image only contains language runtimes plus the worker binary.

If signature or hash verification fails, the worker rejects the artifact, logs the failure, reports it to the scheduler, and does not retry against the same registry endpoint until an operator investigates.

---

## 9. Recall Handling

When the worker dispatches a block, it consults the local block index.  If the index entry is older than a configurable freshness window, the worker re-checks the registry for the current state of that collection version.

If the state is `recalled`, the worker:

1. Refuses to execute the invocation
2. Reports the failure to the scheduler with a "recalled" reason
3. Invalidates the local block index entry for that version
4. Removes the local install from `~/.spade/blocks/<collection>/<version>/`

The scheduler treats this like any other block failure (see `worker.md`'s error handling) and halts the affected pipeline.

Operationally, security recalls are paired with a worker fleet flush so that no in-flight invocation continues running recalled code.  The queue redelivery semantics that make this safe are a concern of the scheduler/queue layer, not the registry; the registry's contract is just "after a recall, no new invocations of this version may execute."

---

## 10. Metadata Mirror (PostgreSQL)

The web UI's "browse blocks" feature does not query the registry directly on every request.  Instead, the system's PostgreSQL database (replacing PocketBase) maintains a **read mirror** of registry metadata:

- Collection name, version, language, state
- Block names, kinds, descriptions, input/output declarations
- Publish timestamps, screening status, signing status

The registry pushes updates to the database on every state transition.  The registry remains the source of truth for artifacts and current state; the database is a low-latency read replica for browse, search, and pipeline editor lookups.

If the database falls out of sync, it can be rebuilt by replaying the registry's audit log or by re-fetching all collection metadata from the registry.

---

## 11. API Surface

Sketch of the HTTP/JSON API.  Specifics (path versioning, error formats) are an implementation concern.

| Endpoint                                                  | Auth          | Purpose                                                     |
| --------------------------------------------------------- | ------------- | ----------------------------------------------------------- |
| `POST /publish`                                           | Developer     | Submit `(repo_url, sha, collection, version)` for ingest    |
| `GET /collections`                                        | Public/Worker | List collections                                            |
| `GET /collections/<name>/versions`                        | Public/Worker | List versions and their states                              |
| `GET /collections/<name>/<version>/status`                | Developer     | Get screening/build status and logs for a publish           |
| `POST /collections/<name>/<version>/state`                | Developer/Op  | Transition state (deprecate, yank, recall)                  |
| `GET /artifacts/<name>/<version>/<platform>/<arch>`       | Worker        | Download the artifact tarball                               |
| `GET /artifacts/<name>/<version>/<platform>/<arch>.sig`   | Worker        | Download the signature                                      |
| `GET /pubkeys`                                            | Worker        | Fetch the current list of trusted public keys               |

State-transition authorization:

- A developer may transition only versions of collections they own
- `deprecated` and `yanked` transitions are available to collection owners
- `recalled` transitions may require operator-level authorization, configurable per deployment

---

## 12. Out of Scope

The following are explicitly **not** part of this spec and are handled elsewhere:

- The screening pipeline implementation (separate per deployment)
- The CI/build infrastructure used to run the build step (e.g. Kubernetes, AWS CodeBuild)
- The blob storage backend used to store artifacts (e.g. S3, MinIO)
- The work-queue/scheduler architecture used to flush workers on recall
- The local block index schema (covered in `worker.md`)
- The CLI's command surface (covered in `cli.md`)
