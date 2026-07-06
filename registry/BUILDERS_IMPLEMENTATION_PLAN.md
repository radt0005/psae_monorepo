# Language Builders — Implementation Plan

This plan extends the Plugin Registry's build step from one real reference
builder (Go) to all five supported languages: **Go, Rust, TypeScript (Bun),
Python, and R**. It picks up where `IMPLEMENTATION_PLAN.md` §12.2 left off
("Python/R/Rust/TS builders are interface stubs returning 'unimplemented'").

The `Builder` interface, the `builder.Run` flow (clone → screen → build →
package → upload staging → complete), the deterministic packager, the control
plane's verify-hash-then-sign, and worker fetch/verify are **already done and do
not change**. This plan only fills in the per-language `Build` implementations
and their container images.

## 0. What "build" must produce (the contract)

The single hard constraint: the artifact must **run on the worker after being
unpacked to a different path** than it was built at. The worker unpacks to
`~/.spade/blocks/<collection>/<version>/` and invokes exactly what
`core.ResolveEntrypoint` (`core/block.go`) dictates — the builder does not get to
choose the run command:

| Language | Worker run command (`ResolveEntrypoint`) | Artifact must contain |
|---|---|---|
| Go | `<dir>/<collection> <entrypoint>` | single binary named `<collection>` + `blocks/` |
| Rust | `<dir>/<collection> <entrypoint>` | single binary named `<collection>` + `blocks/` |
| TypeScript | `<dir>/<collection> <entrypoint>` | single compiled executable named `<collection>` + `blocks/` |
| Python | `uv run --project <dir> --no-sync -m <module>.<entrypoint>` | source tree + `uv.lock` + a **relocatable** `.venv` (deps already installed) + `blocks/` |
| R | `Rscript <dir>/R/<entrypoint>.R` | source tree + `renv.lock` + a **relocated** `renv` library (deps already installed) + `blocks/` |

`registry.md` §5 groups these as: compiled/Bun → "single binary + `blocks/*.yaml`";
Python → "source tree + uv lockfile + populated dependency cache"; R → "source
tree + renv.lock + populated renv library". The operative test in every case is:
after `PackageTarGz` → unpack elsewhere → the `ResolveEntrypoint` command runs a
block **offline** (no toolchain, no network — the worker base image has only
runtimes: `python3`, `R`, GDAL, `uv`, `bun`-compiled binaries are self-contained).

Every builder keeps the existing shared plumbing:
- Target `linux/amd64` (`builder.TargetPlatform`/`TargetArch`).
- Copy `blocks/*.yaml` next to the payload via the existing `copyBlocksDir`.
- Return an `artifactDir`; `builder.Run` packages + hashes + uploads it unchanged.
- `core.DiscoverBlocks`/`LoadBlockManifest` already collect block metadata; no
  per-language manifest work is needed.

---

## Phase A: RustBuilder  *(this session)*

Rust mirrors Go: compile to a single binary, drop it next to `blocks/`.

- [ ] `RustBuilder.Build`: run `cargo build --release --message-format=json` in
      `srcDir`. Parse the JSON line stream for `compiler-artifact` records whose
      `executable` field is non-null; that path is the produced binary. (A Spade
      collection is "a single binary with subcommands", so expect exactly one;
      if several, prefer the bin target matching the package name, else error.)
- [ ] Copy that executable to `artifactDir/<collection>` (mode 0755) and
      `copyBlocksDir(srcDir, artifactDir)`.
- [ ] Native deps (GDAL, PROJ, Arrow via the `data`/`base` collections) link
      **dynamically** against the system libraries that are present in the worker
      runtime image — this is safe because the bundler image is the worker base
      image + toolchains (`registry.md` §5), so the shared-object versions match.
      Do **not** attempt a fully static musl build; it would diverge from the
      worker's glibc/GDAL.
- [ ] Wire `core.CollectionLanguageRust` → `RustBuilder{}` in `BuilderFor`.
- [ ] **Tests:** `testutil.NewRustCollectionRepo` (a real `cargo` crate with one
      subcommand-dispatching `main` + a `blocks/greet.yaml`); build it, assert
      the binary exists, is executable, and running `<bin> greet` emits the
      fixture output; assert `blocks/greet.yaml` is present. Skips if `cargo` is
      absent (matches the Go test's `go` skip).

## Phase B: BunBuilder (TypeScript)  *(this session)*

Bun compiles to a single self-contained executable, so the runtime needs no
`node`/`bun` on the worker — same shape as Go/Rust.

- [ ] `BunBuilder.Build`: `bun install` (resolve deps), then
      `bun build <entry> --compile --outfile <artifactDir>/<collection>`.
- [ ] Resolve `<entry>` from `package.json` (`module` then `main`), falling back
      to `src/index.ts` then `index.ts`; error clearly if none exists. The entry
      is the collection's dispatcher that reads `process.argv` for the block
      subcommand (parallel to the Go fixture's `main`).
- [ ] `copyBlocksDir(srcDir, artifactDir)`.
- [ ] Wire `core.CollectionLanguageTypeScript` → `BunBuilder{}` in `BuilderFor`.
- [ ] **Tests:** `testutil.NewBunCollectionRepo` (a `package.json` collection with
      an `index.ts` dispatcher + `blocks/greet.yaml`); build, run `<bin> greet`,
      assert output + manifest. Skips if `bun` is absent.

## Phase C: Shared updates for A/B  *(this session)*

- [ ] Update `TestBuilderForLanguages`: `rust` and `typescript` are now real
      builders (assert type, not stub-error); `python` and `r` remain stubs.
- [ ] `Dockerfile.builder-rust`: worker base + `rustup`/`cargo` + `git` + GDAL/PROJ
      dev headers + the `cmd/builder` binary. `ENTRYPOINT ["/usr/local/bin/builder"]`.
- [ ] `Dockerfile.builder-ts`: worker base + `bun` + `git` + the builder binary.
- [ ] Extend `BUILDER_IMAGES` guidance in `README.md`/compose to
      `go=…,rust=…,typescript=…` and note the images build in compose.
- [ ] `go test ./... && go vet ./...` green.

---

## Phase D: PythonBuilder  *(done)*

The hard part is a **relocatable** virtualenv: the `.venv` is created in the
build container but must work after unpack at `~/.spade/blocks/...`. The worker
runs `uv run --project <dir> --no-sync` (`--no-sync` ⇒ the env must already be
complete; the worker never touches the network or resolves deps).

Resolved design (validated empirically, `build_python.go`):

- [x] `PythonBuilder.Build`: copy the cloned source tree into `artifactDir`
      (excluding VCS/prior env dirs), then `uv sync --frozen --no-editable
      --no-dev` with `UV_PROJECT_ENVIRONMENT=<artifactDir>/.venv`.
- [x] **Relocatability via non-editable install.** `--no-editable` copies the
      collection package into the venv's `site-packages` instead of leaving an
      absolute `.pth` pointing at the build path, so `uv run --no-sync -m
      <pkg>.<block>` resolves after the artifact moves. (An editable install —
      what local `spade install` uses — is *not* relocatable; the registry path
      must differ here.)
- [x] **Interpreter pin = system python3.** `UV_PYTHON_PREFERENCE=only-system`
      keeps the venv on the image's `python3`; because the bundler image is the
      worker base image + toolchains, that interpreter is at the same absolute
      path on the worker (`pyvenv.cfg home=/usr/bin`), so the venv resolves after
      relocation. This mandates a **shared, pinned Python minor version** between
      the bundler and worker images.
- [x] Generate `uv.lock` if absent (real collections commit it; `--frozen`
      builds exactly the screened lock).
- [x] **Tests:** `TestPythonBuilderBuildsRelocatableVenv` — build the fixture,
      **move** the artifact, run `uv run --project <moved> --no-sync -m
      hello.greet` offline (`UV_OFFLINE=1`) and assert output. Skips if `uv` is
      absent.
- [x] Real `Dockerfile.builder-python`: python3 + uv + GDAL/PROJ dev headers.

### Remaining Phase D risk (collection-authoring, not builder)
- Real geospatial collections reference the `spade` runtime lib as a **monorepo
  path source** (`gdal/pyproject.toml`: `spade = { path = "../../libs/python" }`).
  The registry shallow-clones only the collection, so that path is absent and
  `uv sync` would fail. Publishable collections must reference `spade` (and
  siblings) as a **published/indexed package**, not a monorepo path. This is a
  collection/publishing contract — track it with `cli.md`'s `spade publish`
  preconditions, not the builder.
- Whether GDAL-bindings wheels load against the worker's system GDAL after
  relocation, or need the venv's bundled shared libs — verify with a real
  geospatial collection once one is publishable standalone.

## Phase E: RBuilder  *(done — builder side; worker consumption deferred)*

R ships source + a **populated library baked into the artifact**. The worker has
no R toolchain, so deps cannot be installed at unpack time.

This repo declares R deps two ways, so the builder supports **both**
(`build_r.go`), always installing into an artifact-local library at
`<artifact>/renv/library`:

- [x] `renv.lock` lists packages ⇒ `renv::restore(lockfile, library=<lib>)`.
- [x] else `setup.R` exists ⇒ run it with `R_LIBS_USER`/`R_LIBS` = `<lib>` so its
      `install.packages(lib = user_lib)` lands in the artifact (matches
      `blocks/stats/setup.R`).
- [x] else base-R-only ⇒ ship the source tree as-is.
- [x] Relocatability: R packages relocate across paths when the R minor version +
      platform match (compiled `.so`s link the worker's system libs). Mandates a
      **pinned R version** shared by the bundler and worker images.
- [x] **Tests:** `TestRBuilderBundlesLibrary` — build the fixture (empty
      `renv.lock` + `setup.R` writing a marker into the lib), assert the marker
      landed in `<artifact>/renv/library` (proving the redirect), then move the
      artifact and run `Rscript <moved>/R/greet.R` offline. Skips if `Rscript`
      is absent.
- [x] Real `Dockerfile.builder-r`: R 4.6.0 + build-essential + dev headers + renv.

### Deferred: worker-side library discovery *(coordinated follow-up)*
The worker's `core/executor.go` currently puts only the **host** `~/R` library on
R's search path (`R_LIBS_USER`), not the collection's shipped library, and runs
`Rscript` with CWD=`/work` (so renv's `.Rprofile` autoload never fires). For a
registry-built R artifact to actually run on the worker, the executor must
prepend `<InstalledPath>/renv/library` to R's search path (env-based `R_LIBS`,
alongside the existing `R_LIBS_USER`, is the smallest change). This is an
additive worker change filed as a follow-up — the builder produces the correct
self-contained artifact today (mirrors how the Go builder landed before the live
worker-fetch path).

---

## Cross-cutting

- **Integration test:** once Rust/TS land, extend `internal/integration` (or add
  siblings) to run the full `publish → build → sign → store → fetch → verify`
  chain against the Rust and Bun fixtures, mirroring the existing Go e2e. Python
  and R join when Phases D/E land.
- **Image/runtime version pinning:** Phases D and E both depend on the bundler
  and worker images sharing an exact Python minor / R minor version. Track this
  as a coordinated change with `worker.md`'s base image, not a registry-only one.
- **No interface changes:** `Builder`, `builder.Run`, packaging, signing, and the
  complete/fail callbacks are untouched — this is purely additive per language.

## Sequencing

1. **A + B + C** (Rust, TS, images, tests) — self-contained, mirrors the proven
   Go path. *(done)*
2. **D** (Python) — relocatable non-editable venv on the pinned system
   interpreter. *(done)*
3. **E** (R) — self-contained artifact library (renv or setup.R). *(done —
   builder side; the worker `core/executor.go` R library-path change is the one
   remaining coordinated follow-up.)*

All five language builders are now real. The remaining open items are
cross-component, not builder work: the Python `spade`-as-published-package
publishing contract, and the worker executor's R library-path change.
