# Making Python and R blocks installable through the registry

Status of the end-to-end chain for Python and R collections:

```
publish → screen → build → sign → store → fetch → verify → unpack → index → run
          [-------- DONE (builders) --------]    [--------- GAPS below ---------]
```

The **builders** are done: `PythonBuilder` produces a relocatable non-editable
`.venv`; `RBuilder` bakes an artifact-local R library (via `renv::restore` or
`setup.R`). What remains is everything *downstream* of the build plus a few
authoring/image contracts. Several gaps are cross-cutting (they block all
languages) but Python and R hit them hardest because they ship a runtime
environment, not a single static binary.

---

## A. Cross-cutting prerequisites (block Python **and** R)

### A1. Implement the worker registry-fetch installer  — ✅ DONE
> Implemented per [`runner/INSTALLER_IMPLEMENTATION_PLAN.md`](runner/INSTALLER_IMPLEMENTATION_PLAN.md)
> (Phases 1–8, 10). `core/artifact.go` (Unpack + ed25519 verify + hash) and the
> `BlockRegistryEntry` provenance fields; registry `…/:arch/meta` endpoint;
> `runner/installer` (Client, Installer, PubKeyCache) fetch→verify→unpack→index;
> worker `WithInstaller`/`WithFreshness` do miss→install→run and recall→evict, with
> the security-vs-transient failure split (block failure + poison vs infra nack).
> `spade-worker` reads `REGISTRY_URL`/`SPADE_WORKER_TOKEN`; `seed-blocks` stays the
> fallback. Remaining is operational: provisioning the worker token and web-UI
> authoring-time version pinning (both out of scope). All the numbered steps below
> are covered:
Today the worker's "registry" is the **local SQLite block index**
(`core.OpenRegistry`, `SPADE_REGISTRY=registry.db`), and blocks are installed by
the `seed-blocks` compose shim running `spade install` into a shared volume.
`REGISTRY_URL` is wired in compose but unused. The fetch-and-unpack path in
`worker.md` §8 / `registry.md` §8 does not exist yet. Needs, in `runner/`:
1. On a cache miss for `<collection>/<version>`, `GET /artifacts/.../amd64.tar.gz`
   and `.sig` from `REGISTRY_URL` (worker service token auth).
2. Verify the ed25519 signature against `GET /pubkeys` (refreshed on a schedule).
3. Verify the content hash matches the registry metadata.
4. Unpack into `~/.spade/blocks/<collection>/<version>/`.
5. Insert a block-index row (install source `registry`, signature, fetch-time state).
6. Recall/freshness re-check (`worker.md` §Recall): on `recalled`, refuse + remove.

Until this exists, nothing installs "through the registry" — only via `seed-blocks`.

### A2. Make the packager preserve symlinks — ✅ DONE
`PackageTarGz` now `Lstat`s each entry and emits `tar.TypeSymlink` with `Linkname`
for symlinks (deterministic mtime/sort preserved); `core.Unpack` recreates them on
the worker side (traversal-safe). Covered by `TestPackageTarGzPreservesSymlinks` and
`core.TestUnpackFilesAndSymlinks`. Original description:

`registry/internal/builder/package.go::PackageTarGz` walks files, `os.Stat`s and
`os.Open`s each path (both **follow** symlinks), and writes plain
`tar.TypeReg` entries. Consequences:
- A **file** symlink (e.g. `.venv/bin/python → /usr/bin/python3`) becomes a full
  byte copy — bloats the artifact and can break venv/interpreter detection.
- A **directory** symlink (e.g. `.venv/lib64 → lib`, or renv-cache links) makes
  `io.Copy` fail with "is a directory" → **the build errors at packaging**.

Fix: in `PackageTarGz`, `Lstat` each entry; for symlinks emit `tar.TypeSymlink`
with `Linkname` (keep deterministic mtime/sort). The worker's untar (A1) must
recreate symlinks. Absolute targets like `/usr/bin/python3` resolve on the
worker because the bundler is the worker base image + toolchains (see A3).
*Alternative:* build fully copied venvs/libraries so no symlinks remain — harder
with uv (the interpreter links are not trivially copyable), so prefer fixing the
packager. **Note:** the current builder unit tests relocate with `os.Rename`
(symlink-preserving) and so do **not** exercise this; add a tar→untar→run test.

### A3. Pin bundler and worker to the same base image / runtime versions — ✅ DONE
`registry.md` §5: the bundler image must be "the worker base image plus
toolchains" so glibc, system libs, and the **interpreter minor version** match.
Status:
- **R — done.** `registry/Dockerfile.builder-r` rebased on `ubuntu:24.04` + apt
  `r-base` (= the worker's R **4.3.3** on noble) with `PPM_CODENAME=noble` serving
  matching binaries. Verified by a real image build.
- **Python — done.** `registry/Dockerfile.builder-python` rebased on `ubuntu:24.04`
  → system Python **3.12.3** at `/usr/bin/python3` (= the worker), `uv` +
  `UV_PYTHON_PREFERENCE=only-system` unchanged. Verified by a real image build. So
  a shipped venv's base-interpreter links and `cp312` wheel tags resolve after
  relocation.
- Worker (`runner/worker/Dockerfile`) = `ubuntu:24.04` → Python **3.12.3**, R **4.3.3**.
- Remaining (C4-style): confirm any system libs wheels/R packages link against
  (GDAL/PROJ) match between bundler and worker; same base image makes this likely
  but it is not yet explicitly verified per-package.

A venv/library built against one interpreter minor will not resolve against a
different one after relocation (`pyvenv.cfg home=/usr/bin` points at the wrong
python; compiled R `.so`s mismatch). **Rebuild the bundler images `FROM` the
worker base (`ubuntu:24.04`) + language toolchains**, or otherwise pin identical
Python/R minors across both images and keep them in lockstep.

---

## B. Python-specific steps

1. **Publishable collections (authoring contract).** Real collections reference
   the runtime lib as a monorepo path source
   (`blocks/gdal/pyproject.toml`: `spade = { path = "../../libs/python" }`). The
   registry shallow-clones only the collection, so that path is absent and
   `uv sync` fails. Required:
   - Publish `spade` (and any sibling libs) to an index (PyPI or an internal
     index/`--find-links`).
   - Collections depend on the **published** `spade>=x`; drop the
     `[tool.uv.sources]` path override.
   - Commit a `uv.lock` that resolves standalone (enforced by `spade check`/
     `spade publish`, `cli.md`).
2. **Builder** — done (`build_python.go`): `uv sync --frozen --no-editable
   --no-dev`, `UV_PYTHON_PREFERENCE=only-system`. Non-editable install is what
   makes the venv relocatable (no absolute `.pth`).
3. **Packaging** — depends on A2 (venv symlinks) and A3 (Python 3.12 match).
4. **Worker execution** — `core.ResolveEntrypoint` already emits
   `uv run --project <dir> --no-sync -m <module>.<block>`, and `core/executor.go`
   already binds `uv`, the InstalledPath, and caches into the sandbox. Verify it
   works against a **non-editable, system-python** venv (the seed path today is
   an *editable* venv on a uv-managed python via `UV_PYTHON_INSTALL_DIR`; the
   registry venv differs). `python3` is on the worker at `/usr/bin` — no managed
   interpreter needed once A3 aligns minors.
5. **Native deps.** The worker image installs `python3` but **no system GDAL**;
   geospatial collections currently rely on the `gdal` wheel's bundled libs
   (girder `find-links`). Confirm those wheels load on the worker after unpack;
   if any dep links the *system* GDAL, add it to the worker image (and match its
   version in the bundler).
6. **Test.** Extend `internal/integration` to build the Python fixture →
   `PackageTarGz` → untar elsewhere → `uv run --no-sync -m ...` offline.

---

## C. R-specific steps

1. **Builder** — done (`build_r.go`): installs into `<artifact>/renv/library`
   via `renv::restore` (if `renv.lock` lists packages) else `setup.R` (with
   `R_LIBS_USER`/`R_LIBS` redirected into the artifact).
   - If the **renv path** is used, **disable the renv cache** during restore
     (`RENV_CONFIG_CACHE_ENABLED=FALSE` / `renv::settings$use.cache(FALSE)`) so
     the shipped library holds **real package files**, not symlinks into a global
     cache — otherwise packaging (A2) omits/breaks them. The `setup.R` path uses
     `install.packages`, which already writes real copies.
2. **Worker executor change (deferred follow-up).** `core/executor.go`
   `languageSandboxBinds` currently puts only the **host** `~/R` library on R's
   search path (`R_LIBS_USER`), and runs `Rscript` with CWD=`/work` (so renv's
   `.Rprofile` autoload never fires). Required: prepend
   `<InstalledPath>/renv/library` to R's search path — set `R_LIBS` from the
   install path, alongside the existing `R_LIBS_USER`, and bind that dir into the
   sandbox. Without this, registry-installed R blocks cannot find their packages.
3. **Version pin** — A3 (R minor must match; compiled `.so`s are version- and
   platform-specific).
4. **Native deps.** Match GDAL/PROJ/system libs the R packages link against
   between bundler and worker (same base image, A3).
5. **Test.** Build the R fixture → `PackageTarGz` → untar elsewhere →
   `Rscript <dir>/R/<block>.R` with the chosen `R_LIBS` mechanism, offline.

---

## D. Recommended order of work

1. **A2 packager symlinks** — small, self-contained in `registry/`, unblocks the
   Python artifact from surviving a real fetch. Add the tar-round-trip test.
2. **A3 image alignment** — rebuild bundler images from the worker base; unblocks
   both Python and R relocation correctness.
3. **A1 worker registry-fetch installer** — the large, shared piece that makes
   "install through the registry" real for every language.
4. **C2 worker R library-path change** — makes R artifacts actually run.
5. **B1 publish `spade` to an index + update collections** — makes real Python
   collections buildable standalone.
6. **B6 / C5 end-to-end tests** through the real tar round-trip (not `os.Rename`).

Items 1–2 and 4 live in this repo's `registry/` and `core/`; item 3 is in
`runner/`; item 5 is collection-authoring + a package-publishing step.
