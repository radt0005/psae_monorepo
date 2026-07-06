# Adopting `pak` in the R collection builder

Plan to replace the `renv`-based install engine in `RBuilder`
(`registry/internal/builder/build_r.go`) with [`pak`](https://pak.r-lib.org),
keeping the "self-contained, relocatable artifact library" contract from
`registry.md §5` and `notes.md §C`.

Companion to `BUILDERS_IMPLEMENTATION_PLAN.md` and `notes.md`; cross-references
their item numbers (A1–A3, C1–C5) rather than restating them.

---

## 1. Goal & scope

**Goal.** Use `pak` as the mechanism that installs a collection's R dependencies
into the artifact-local library (`<artifact>/renv/library` today), replacing
`renv::restore` and the `install.packages` calls inside `setup.R`.

**Why pak (recap).** Mature (r-lib/Posit), PPM binary installs (faster than
renv's source compiles), and — decisively for us — it installs **real package
files** into the target `lib`, so the artifact packages cleanly without the
cache-symlink workaround `notes.md §C1` needs for renv.

**In scope:** `build_r.go`, `Dockerfile.builder-r`, the R fixtures/tests in
`internal/testutil` + `builder_test.go`, and a pak-native dependency manifest for
collections (`blocks/stats`, `blocks/sae`).

**Out of scope (tracked elsewhere, but gating — see §7):** worker R search-path
change (`notes.md §C2`), packager symlink support (`A2`), bundler/worker R-minor
alignment (`A3`), publishing the `spade` R lib (parallels Python `B1`).

---

## 2. Where we are today

`RBuilder.Build` installs into `<artifact>/renv/library` via one of two branches
(`build_r.go`):

1. `renv.lock` lists packages ⇒ `renv::restore(lockfile, library=libDir)`.
2. else `setup.R` exists ⇒ run it with `R_LIBS_USER`/`R_LIBS` redirected into the
   artifact (`rEnv(libDir)`).

**Reality check:** both real collections ship `renv.lock` with `"Packages": {}`,
so branch 1 never fires. `setup.R` does everything: installs `jsonlite`, `yaml`,
and builds the local `spade` package from `../../libs/R`
(`blocks/stats/setup.R`). Any pak adoption that only rewrites branch 1 changes
nothing in practice — the `setup.R` path is the one that matters.

---

## 3. Design decisions

**D1 — Dependency source of truth.** *(decided)* Two supported paths:
1. **Primary (examples use this):** a per-collection `DESCRIPTION` with an
   `Imports:` field (the R-standard, and pak resolves it directly via `deps::.`),
   plus a committed `pkg.lock` from `pak::lockfile_create("deps::.")` for pinned
   reproducibility. Mirrors the Python model (`pyproject.toml` + `uv.lock`,
   `build_python.go`).
2. **Fallback (kept as a first-class option):** `setup.R` using
   `pak::pkg_install(...)` for its installs. Not just an escape hatch for
   arbitrary build logic — a supported, pak-powered path so authors who prefer
   imperative setup keep binary installs and the clean-library guarantee (D4).
   The builder routes `setup.R`'s installs through pak, not `install.packages`.

**D2 — Lockfile format.** Use pak's own `pkg.lock`, not `renv.lock`. pak's
`lockfile_install` reads pak lockfiles; don't try to feed it a renv lockfile.
`renv.lock` is dropped from the R collection contract (update `spade init` and
docs — §6).

**D3 — Binary installs (PPM).** Point `repos` at the PPM **Linux binary**
endpoint whose distro codename matches the worker base image
(`A3`): ubuntu 24.04 ⇒ `.../cran/__linux__/noble/latest`. pak sets the HTTP
user-agent so PPM serves binaries. Codename mismatch silently falls back to
source — so this is coupled to `A3` and must be asserted, not assumed.

**D4 — Cache vs. library.** pak's download/metadata cache (`PKGCACHE_DIR`) is
**separate** from the installed `lib`; pak copies real package trees into `lib`.
So `notes.md §C1`'s "disable the renv cache" step is unnecessary. Still add a
symlink-scan assertion to the round-trip test to lock this guarantee in.

**D5 — R version.** pak installs against whatever R runs it; it does **not** fix
`A3`. Keep pinning R via the bundler base image and keep it equal to the worker's
R minor (compiled `.so`s are version/platform-specific).

**D6 — The local `spade` R package.** *(decided — interim: local install)*
`setup.R` installs it from `../../libs/R`, absent after the registry's shallow
clone (same class of gap as Python `B1`). Publishing `spade` (CRAN or an internal
repo) is **deferred** — CRAN submission has a long lead time. Interim: a
special-cased `pak::local_install("libs/R")` step (guarded by dir existence) for
the monorepo layout, so `library(spade)` resolves without a published package.
Revisit publishing later; when it lands, `spade` moves into `Imports:` and the
special-case is removed.

---

## 4. Phases

### Phase 0 — Validation spike ✅ DONE — GO
Ran locally against `blocks/stats` deps (`jsonlite`, `yaml`) + local `spade`, with
R 4.6.0 / pak 0.9.5 / PPM `jammy`. Script: `scratchpad/pak_spike.sh`. Results:
- [x] **Real files, no symlinks.** `find <lib> -type l` = 0. pak's cache is separate
      from `lib`; packages are real trees ⇒ `notes.md §C1` renv-cache workaround is
      unnecessary.
- [x] **PPM served binaries.** `jsonlite`/`yaml` came as
      `x86_64-...-ubuntu-22.04` artifacts, installed in <40 ms with no build step;
      `pkg.lock` records `"platform": "...ubuntu-22.04", "rversion": "4.6"`. (`spade`
      built from source, as expected for a local pkg.)
- [x] **`.so`s relocate.** After copying the lib to a different absolute path,
      `jsonlite`/`yaml`/`spade` all `library()`-loaded from the moved path.
- [x] **Deterministic reinstall** from the committed `pkg.lock` reproduced the same
      package set. (`spade` differs by design — it's installed via the D6 local step,
      not the lockfile.)

Two refinements folded into Phase 1 (see below):
- **R0-a:** pak drops a `<lib>/_cache` staging dir (install locks) — real files, no
  runtime use. **Remove it before packaging** so it doesn't bloat the artifact.
- **R0-b:** `lockfile_create` stored an **absolute** `deps::/abs/build/path` top-level
  ref. Run it with **CWD=artifactDir and ref `deps::.`** so the committed `pkg.lock`
  is path-portable.

A3 coupling confirmed empirically: binaries are distro-codename-tagged
(`ubuntu-22.04` here). The bundler must run on the **worker's** codename (target
`noble`/24.04) to get matching binaries — verify PPM has `noble` + R 4.6 binaries.

### Phase 1 — pak as the install engine in `build_r.go` ✅ DONE
Kept `RBuilder.Build`'s shape and the `<artifact>/renv/library` layout
(renaming the dir is a separate, worker-coupled change — left for §7/C2).
Implemented in `build_r.go`: `pakInstall` (pkg.lock verbatim, else
`lockfile_create("deps::.")` then install), `pakInstallSpade` (D6, gated on
`SPADE_R_LIB_SRC`), `_cache` removal (R0-a), CWD=artifact so the ref is `deps::.`
(R0-b). Branch order is pak → setup.R → legacy renv → base-R. Verified by
`TestRBuilderPakBundlesLibrary` (Phase 4).
- [x] Add a `pak.lock`/`DESCRIPTION` detection branch **ahead of** the renv/setup
      branches:
  - `pkg.lock` present ⇒
    `pak::lockfile_install(lockfile="pkg.lock", lib=libDir)`.
  - else `DESCRIPTION` present ⇒ `pak::lockfile_create("deps::.")` then install
    (or `pak::local_install_deps(root=".", lib=libDir)`), so a missing lock still
    builds reproducibly-enough (parallels `build_python.go`'s "uv lock if
    absent").
- [ ] Run via the existing `runInEnv(ctx, artifactDir, rEnv(libDir), "Rscript",
      "-e", script)` helper; extend `rEnv` to also set `PKGCACHE_DIR` to a
      build-scoped temp and `repos` to the PPM binary endpoint (D3).
- [ ] `setup.R` stays a **supported, pak-powered fallback** (D1.2): run it with the
      PPM-configured env so `pak::pkg_install` inside it gets binaries and lands in
      `libDir`. Keep `renv::restore` only as a legacy rollback branch.
- [ ] `spade`-lib handling per D6 (interim: `pak::local_install("libs/R")` guarded
      by dir existence — publishing deferred).
- [ ] **R0-a:** after install, remove `<libDir>/_cache` before returning the artifact
      (pak install-lock staging; not needed at runtime).
- [ ] **R0-b:** generate `pkg.lock` with `runInEnv(ctx, artifactDir, ...)` (CWD =
      artifact) and ref `deps::.`, so the stored top-level ref is relative/portable.

Sketch:
```go
switch {
case fileExists(filepath.Join(artifactDir, "pkg.lock")):
    script := fmt.Sprintf(`.libPaths(%q); pak::lockfile_install(%q, lib=%q)`,
        libDir, "pkg.lock", libDir)
    // runInEnv(... rEnv(libDir) ... "Rscript","-e",script)
case fileExists(filepath.Join(artifactDir, "DESCRIPTION")):
    // pak::lockfile_create("deps::.") -> pak::lockfile_install(lib=libDir)
case fileExists(filepath.Join(artifactDir, "setup.R")):
    // supported fallback: run setup.R with PPM env; installs go through
    // pak::pkg_install (D1.2), captured into libDir via rEnv(libDir).
case populated: // legacy renv.lock — rollback-only branch
    // renv::restore(...)  (unchanged)
}
```
Note the ordering change: `setup.R` now precedes the legacy `renv.lock` branch,
since setup.R is a first-class pak path and renv is rollback-only.

### Phase 2 — Bundler image (`Dockerfile.builder-r`) — mostly done
- [x] Install pak into the image, **alongside** `renv` (legacy rollback branch).
- [x] Set `repos` via `Rprofile.site` so the builder and any `setup.R` inherit it.
      Uses a `PPM_CODENAME` build ARG: set ⇒ P3M binary endpoint (D3); unset ⇒
      source CRAN. Left unset for now because the base is Debian bookworm, not a
      P3M-binary distro — flip to `noble` under A3 (see below).
- [ ] **Deferred to A3:** once the base is rebased on the worker
      (`ubuntu:24.04`) and `PPM_CODENAME=noble` serves binaries, **trim**
      `build-essential` + dev headers for binary-available packages, keeping them
      for source-only packages (GDAL/`sf`; verify per `C4`). Document which stay.
- [ ] **A3:** keep the R base image tag equal to the worker's R minor.

### Phase 3 — pak-native manifests + collection migration ✅ DONE
- [x] Added `DESCRIPTION` + committed `pkg.lock` to `blocks/stats`
      (`Imports: jsonlite`) and `blocks/sae` (`Imports: yaml`); deleted their
      `renv.lock` and `setup.R`. `spade` is not in `Imports` (unpublished) — it and
      its transitive deps (`yaml`/`methods`) install via the local-spade step.
      Locks use **source** CRAN refs (`platform: source`, ref `deps::.`) so a
      committed lock is portable across build hosts and reproducible; binaries stay
      an install-time choice once an image configures PPM (they'd require
      regenerating the lock on the target codename — the source lock always
      compiles).
- [x] `spade install` (`cli/cmd/install.go` `runBuild`) now prefers the pak path
      for R (`DESCRIPTION`/`pkg.lock` → install deps + local `spade` into the user
      library), with `setup.R` as the fallback and no-op when neither is present.
- [x] `spade init` scaffolds `DESCRIPTION` (no lock — generated at build, like
      Python's `uv.lock`); `spade upload` ships `DESCRIPTION` + optional
      `pkg.lock`/`setup.R` (`cli/cmd/{init,upload}.go`, tests updated).
- [x] Docs/specs updated: `spec/registry.md §5`, `spec/hosting.md §89`,
      `documentation/.../cli/init.md`, `.../libraries/r/_index.md`,
      `registry/README.md`, and the stats collection README/SPECIFICATION.

### Phase 4 — Tests ✅ DONE (for the pak path)
- [x] New R fixture `NewRPakCollectionRepo` in `internal/testutil/harness.go`: a
      `DESCRIPTION`-based collection with a real compiled dep (`jsonlite`) so the
      round-trip exercises `.so` relocation. The setup.R fixture stays for the
      fallback path.
- [x] `TestRBuilderPakBundlesLibrary` asserts: dep installed under
      `<artifact>/renv/library`, `_cache` removed (R0-a), **no symlinks** in the
      library (D4), and `pkg.lock` holds a relative `deps::.` ref with no build
      path leaked (R0-b).
- [x] True tar round-trip (`notes.md §C5`): build → `PackageTarGz` → `tar xzf` to a
      new path → `Rscript <dir>/R/greet.R` with `R_LIBS` at the shipped library.
      This exercises the packager (`A2`) — the pak library is symlink-free, so
      `PackageTarGz` ships it as-is. (The setup.R test still uses `os.Rename`.)
- [x] Skips when `Rscript` **or** `pak` is unavailable.
- Full suite green: `go test ./...` (builder incl. the 11s pak round-trip).

---

## 5. Documentation & spec updates ✅ DONE (Phase 3)
- [x] `spec/registry.md §5` → pak dependency manifest (`DESCRIPTION`/`pkg.lock`) + library.
- [x] `spec/hosting.md §89` (`R` + `renv`) → `R` + `pak`.
- [x] `documentation/.../cli/init.md` marker-file table + scaffold tree → `DESCRIPTION`.
- [x] `documentation/content/libraries/r/_index.md` → pak / `DESCRIPTION` Imports.
- [x] `registry/README.md`, plus the stats collection README/SPECIFICATION.
- [ ] `notes.md §C1` cache note is now obsolete for pak (informational; left as-is).

---

## 6. Rollback / fallback
The renv and setup.R branches stay in `build_r.go` through Phases 1–3, selected
only when no pak manifest is present. Reverting is deleting the pak branch and the
image's pak install — no artifact-format change is forced until §7/C2 renames the
library dir. Ship pak behind the manifest so un-migrated collections keep building
on the old path.

---

## 7. Gating cross-cutting items
- **A3** ✅ DONE — `Dockerfile.builder-r` rebased on `ubuntu:24.04` + apt `r-base`
  (the worker's exact base and R install), so both run **R 4.3.3** on noble. Image
  builds; PPM `noble` serves `ubuntu-24.04` binaries (`pak`/`renv`/`jsonlite`
  installed as binaries, no compile). Bundler and worker codenames now match, so
  compiled `.so`s baked by the bundler load on the worker. Dev headers kept for
  source-only packages (GDAL/sf); trimming them is a later size optimization (C4).
- **C2** ✅ DONE — `core/executor.go` `languageSandboxBinds` now sets
  `--env=R_LIBS=<InstalledPath>/renv/library` (the dir is already bound via the
  InstalledPath mount), so registry-installed R blocks find their pak-installed
  packages. Covered by `TestLanguageSandboxBindsRLibs`.
- **A2** — packager must preserve symlinks; pak *reduces* reliance on this (real
  files) and the Phase 4 round-trip confirms the R library ships via `PackageTarGz`
  as-is. Still relevant for other languages' venv/bin symlinks.
- **B1-parallel (D6)** — publishing the `spade` R lib is **deferred**; interim uses
  a local install so `library(spade)` resolves after a shallow clone. Revisit when
  a published (CRAN/internal-repo) `spade` is available.
- **A1** — the worker registry-fetch installer (unpack artifact → set
  `InstalledPath`) is what ultimately drives C2 in production; still unbuilt
  (`notes.md §A1`). Until then C2 is exercised by the unit test, not a live fetch.

---

## 8. Suggested order
0. Phase 0 spike (½ day; answers go/no-go).
1. Phase 2 image (pak + PPM) — needed to run the spike and Phase 1 in-container.
2. Phase 1 builder branch + Phase 4 tests (behind the manifest, renv fallback).
3. Phase 3 collection migration + `spade init` + docs (§5).
4. Coordinate A3/C2 so pak-built artifacts actually run on the worker.
