# Worker Registry-Fetch Installer — Implementation Plan

> **STATUS (implemented).** Phases 1–8 and 10 are done and tested; the prerequisites
> A2 (packager symlinks) and A3 (bundler/worker image alignment) landed alongside.
> Phase 9 (multi-interpreter ABI tag) remains a deferred design note. Full sweep
> (`go test ./...` + `go vet ./...`) is green across core, runner, and registry.
> Key artifacts: `core/artifact.go` (Unpack, VerifySignature, HashMatches) and the
> `BlockRegistryEntry` provenance fields; the registry `…/meta` endpoint; the
> `runner/installer` package (Client, Installer, PubKeyCache); worker wiring in
> `runner/worker/worker.go` (WithInstaller/WithFreshness) and
> `runner/cmd/spade-worker` config + compose. What remains for a *live* production
> path is operational: provisioning a worker service token and authoring-time
> version pinning in the web UI (both out of scope here).

This plan implements the **worker installer**: the fetch-and-unpack path that
lets a worker pull a signed collection artifact from the cloud Plugin Registry,
verify it, unpack it, and run a block — with no build toolchains on the worker
(`worker.md` §Worker Installer, `registry.md` §8). It is item **A1** in
`../notes.md` and the largest remaining gap between "the registry builds and
signs artifacts" and "a worker installs and runs them."

It completes the trust chain on the worker side:

```
… fetch → verify sig → verify hash → unpack → index → run
          [------------------ THIS PLAN ------------------]
```

Today the worker's "registry" is the **local SQLite block index**
(`core.OpenRegistry`, `SPADE_REGISTRY=registry.db`); blocks are installed by the
`seed-blocks` compose shim running `spade install`. `REGISTRY_URL` is wired into
compose but unused. This plan adds the real registry-fetch path and keeps
`seed-blocks` as a fallback.

---

## 0. Decisions (settled in design discussion)

- **Version resolution = Option A: pin the collection version in the
  assignment.** The registry serves artifacts keyed by *collection* version
  (`<collection>/<version>/<platform>/<arch>`), which is distinct from the
  per-block `version:` field in `blocks/*.yaml` (that stays as-is: cache keys via
  `core.ComputeCacheKey`, display). The worker cannot read the in-artifact YAML
  before installing (chicken-and-egg), so the pinned collection version travels
  in the `WorkerAssignment`. Its source of truth is the metadata mirror at
  pipeline-authoring time. Full chain:

  ```
  mirror (collection+version+block) → PipelineBlock.Version (pinned at authoring)
      → WorkerAssignment.CollectionVersion → worker fetch key
  ```

- **Single runtime version fleet-wide (for now).** The R and Python interpreter
  minor versions are pinned identically across the bundler and worker images
  (`notes.md` A3), so the R/Python version "rides along" with the base image and
  `<platform>/<arch>` is a sufficient artifact key. Multiple coexisting
  interpreter versions (folding the interpreter version into an ABI compatibility
  tag, wheel-tag style) is a **deferred** extension — see Phase 9. It is a
  *worker-environment* coordinate, not a pipeline pin, so it does not affect
  Option A.

- **Hook point = the lookup miss** at `runner/worker/worker.go:131`. A miss
  becomes fetch → install → re-lookup instead of a terminal failure. Recall /
  freshness re-check happens on a lookup *hit* (Phase 6).

- **Module layout.** The installer (network, service token) lives in a new
  `runner/installer` package. `core` gains offline, testable primitives: block-
  index schema fields, a symlink-aware untar, and an ed25519 verify helper. The
  registry gains one worker-facing metadata endpoint.

- **Trust primitive = ed25519 verify against a *list* of pubkeys** (rotation,
  `registry.md` §6.1), stdlib `crypto/ed25519`. Verification lives in `core` so
  it is unit-testable without a network; pubkey *fetching/caching* is in `runner`.

## Prerequisites (tracked in `notes.md`, must land alongside)

- **A2 — packager preserves symlinks.** `registry/internal/builder/package.go`
  currently dereferences symlinks (and errors on directory symlinks). Python
  venvs and cache-backed renv libraries contain symlinks, so the untar side of
  this plan and the tar side (A2) must agree. Do A2 first, or in lockstep with
  Phase 2's untar.
- **A3 — bundler/worker image version pinning.** Required for relocatable
  Python/R artifacts to resolve on the worker at all.

---

## Phase 1: Registry worker-facing artifact metadata endpoint

The installer must verify the artifact content hash "against the value recorded
in registry metadata" (`registry.md` §8). Today the hash is exposed only on the
developer/owner-auth `GET /collections/:name/:version/status`
(`wire.VersionStatus`); the worker-facing `GET /collections/:name/versions`
returns only `{Version, State}` (`registry/internal/api/handlers_public.go:44`).

- [ ] Add a worker/public endpoint returning per-artifact metadata for a version:
      `{version, platform, arch, content_hash, state}`. Either extend the
      `versions` response with artifact rows or add
      `GET /collections/:name/:version/:platform/:arch/meta`.
- [ ] Serve it under the same public/worker auth as the other read endpoints.
- [ ] **Tests:** returns the stored `content_hash` + current `state`; 404 for an
      unknown artifact; state reflects transitions (available/yanked/recalled).

## Phase 2: `core` primitives (offline, testable)

### 2.1 Block-index schema additions (`core/types.go` `BlockRegistryEntry`)
The current struct has no field for install provenance or recall bookkeeping.
- [ ] Add `Source` (`registry` | `local`), `Signature` (base64), `RegistryState`
      (last-seen state), `LastVerifiedAt` (time). GORM `AutoMigrate` adds columns;
      existing `seed-blocks` rows leave them empty (`Source=local`), so the
      freshness/recall path (Phase 6) engages only for `Source=registry`.
- [ ] Extend `RegisterBlock` / `LookupBlock` to round-trip the new fields.
- [ ] **Tests:** round-trip; `local` rows are untouched by registry logic.

### 2.2 Symlink-aware untar (`core`)
- [ ] `Unpack(tar.gz reader, destDir)` that recreates regular files **and
      symlinks** (`tar.TypeSymlink`), preserving the executable bit, rejecting
      path traversal (`..`, absolute) — the inverse of the A2 packager.
- [ ] **Tests:** round-trip a tree containing a file symlink and a dir symlink
      (mirrors a venv layout); traversal entries are rejected.

### 2.3 ed25519 verify helper (`core`)
- [ ] `VerifySignature(pubkeys [][]byte, data, sig []byte) bool` — true if *any*
      trusted key verifies (rotation). `HashMatches(data, hexHash) bool`.
- [ ] **Tests:** good/bad key; multi-key (old+new) accepts both; tampered bytes
      fail. (Mirrors `registry/internal/sign` on the producer side.)

## Phase 3: Option A version plumbing (assignment carries the pin)

- [ ] Add `Version` (collection version) to `core.PipelineBlock` (it has only
      `Name` today). Populated by the web editor from the mirror at authoring
      time (upstream; out of scope here, but the field must exist).
- [ ] Add `CollectionVersion` to `core.WorkerAssignment`.
- [ ] Populate it in `runner/build_job.go` `BuildJobForInvocation` from the
      owning `PipelineBlock.Version`.
- [ ] Worker fetch key = `collection` (from `BlockName` before the `.`) +
      `CollectionVersion` + the worker's own `platform`/`arch` (`linux`/`amd64`
      constants, mirroring `builder.TargetPlatform/Arch`).
- [ ] **Back-compat:** empty `CollectionVersion` ⇒ current behavior
      (`LookupBlock(name, "")`, no fetch) so `seed-blocks` and existing tests
      keep working.
- [ ] **Tests:** assignment carries the pinned version; empty version preserves
      legacy lookup.

## Phase 4: The installer (`runner/installer`)

A `Client` (worker service-token auth) + an `Installer` that, given
`(collection, version, platform, arch, destDir)`:

- [ ] `GET /artifacts/<c>/<v>/<platform>/<arch>.tar.gz` and `…/.sig`.
- [ ] Fetch expected `content_hash` from the Phase 1 endpoint; refuse non-servable
      states early (410 `yanked`/`recalled` ⇒ do not install; feed Phase 6).
- [ ] **Verify signature** over the tarball bytes against the cached pubkeys
      (Phase 7) using `core.VerifySignature`.
- [ ] **Verify content hash** with `core.HashMatches`.
- [ ] **Unpack** (`core.Unpack`) into a **temp dir**, then **atomic rename** to
      `~/.spade/blocks/<c>/<v>/` so a crash never leaves a half-install indexed.
- [ ] **Register** one `BlockRegistryEntry` per `blocks/*.yaml`
      (`core.DiscoverBlocks` / `LoadBlockManifest`), with `Source=registry`,
      `Signature`, `RegistryState=available`, `LastVerifiedAt=now`.
- [ ] **Concurrency:** a per-`<c>/<v>` file lock + idempotent "already installed?"
      recheck under the lock, so two jobs needing the same collection fetch once.
- [ ] **Tests (fake registry server serving a signed fixture artifact):** happy
      path installs + indexes; **bad signature** and **hash mismatch** reject and
      leave nothing indexed; 410 short-circuits; concurrent installs fetch once.

## Phase 5: Wire into the worker loop + failure taxonomy

- [ ] Add an optional `Installer` interface to the `worker.Worker` struct (nil ⇒
      today's behavior; keeps the fallback + existing tests green).
- [ ] At `worker.go:131`, on a lookup miss with `Installer != nil` and a non-empty
      `CollectionVersion`: install, then re-`LookupBlock`.
- [ ] **Map failures onto the worker's two-mode convention** (the `worker.go`
      package doc):
  - Bad signature / hash mismatch / `recalled` ⇒ **block failure**
    (`Status=Error`, nil err → publish result + ack). Security failures also set a
    sticky **poison marker** for that `<c>/<v>` so the worker does not re-hit the
    same endpoint until an operator clears it (`worker.md` §8).
  - Registry unreachable / 5xx ⇒ **infrastructure failure** (non-nil err → nack →
    redeliver), so a registry blip doesn't mass-fail pipelines.
- [ ] **Tests:** miss→install→run happy path (fake installer); security failure ⇒
      block failure + poison; transient failure ⇒ infra error (nack).

## Phase 6: Recall + freshness re-check (`worker.md` §9)

- [ ] On a lookup **hit** for a `Source=registry` entry whose `LastVerifiedAt` is
      older than a configurable freshness window, re-check state via the versions/
      meta endpoint before executing.
- [ ] If `recalled`: refuse, report to the scheduler with a `recalled` reason,
      invalidate the index entry, and remove `~/.spade/blocks/<c>/<v>/`.
- [ ] On a successful re-check, bump `LastVerifiedAt` (and `RegistryState`).
- [ ] **Tests:** stale entry triggers a re-check; `recalled` ⇒ refuse + evict +
      reason; fresh entry skips the network.

## Phase 7: Pubkey cache + rotation

- [ ] `GET /pubkeys` on startup and on a refresh timer; persist to a local cache
      file so a registry blip doesn't stall verification.
- [ ] Hold the full **list** of trusted keys; verification accepts any (Phase 2.3)
      so a rotation is flag-day-free (`registry.md` §6.1).
- [ ] **Tests:** cache load/refresh; verify still succeeds mid-rotation (old+new).

## Phase 8: Config + compose wiring (`runner/cmd/spade-worker`)

- [ ] Parse `REGISTRY_URL` (already set in compose), the worker **service token**,
      the pubkey cache path, and the freshness window (flags + env, matching the
      existing `getenv` pattern in `main.go`).
- [ ] Construct the `Installer` and pass it to `worker.New`; leave it nil (legacy
      path) when `REGISTRY_URL` is unset.
- [ ] Provision a worker service token in the registry (bootstrap/env) and set it
      in the compose `worker` service.
- [ ] Keep `seed-blocks` as the documented fallback.
- [ ] **Tests:** worker starts with/without `REGISTRY_URL`; token is sent on fetch.

## Phase 9: Interpreter-version ABI tag (deferred)

- [ ] Only if multiple R/Python minors must coexist: extend the `platform`
      segment of the artifact key into an ABI compatibility tag that encodes the
      interpreter version (wheel-tag style, e.g. `linux-cp312` / `linux-R4.6`),
      built by the bundler and sent by the worker from its own image. Documented
      now; not built. Does not change Option A.

## Phase 10: End-to-end

- [ ] Runner-side integration: a fake (or real, in compose) registry serving a
      **signed** fixture collection → worker miss → install → verify → run a block
      under `isolate`, asserting sig+hash verification and index provenance.
- [ ] Recall path e2e: installed → registry `recalled` → worker refuses + evicts.
- [ ] `go test ./...` + `go vet ./...` green across `core`, `runner`, `registry`.

---

## Out of scope

- Authoring-time version pinning in the web UI (the mirror→`PipelineBlock.Version`
  source) — upstream; this plan only consumes the pinned value.
- The scheduler/queue **fleet-flush** that pairs with a recall (`registry.md` §9)
  — the worker's contract ends at "refuse + evict on recall."
- A2 (packager symlinks) and A3 (image pinning) themselves — prerequisites
  tracked in `notes.md`, not re-specified here.
- Multi-interpreter ABI tags (Phase 9 is a stub/《design note》 until needed).

## Suggested order

1. Prereq **A2** (packager symlinks) — unblocks Python/R round-trips.
2. **Phase 1** (registry meta endpoint) + **Phase 2** (`core` primitives) — small,
   offline, independently testable.
3. **Phase 3** (version plumbing) — threads Option A end to end.
4. **Phase 4** (installer) + **Phase 5** (loop wiring) — the core feature.
5. **Phase 7** (pubkeys) then **Phase 6** (recall) — trust + safety.
6. **Phase 8** (config/compose) + **Phase 10** (e2e). **Phase 9** only if needed.
