# CLI Implementation Plan: `spade install <path>` (local source)

Extend `spade install` so that, in addition to a git URL, it accepts a **local directory path** (including `.`, `./subdir`, and absolute paths). The user can run `spade install .` from inside a collection to install the working tree directly — no git commit or `file://` wrapper required.

Behavior summary:
- `spade install <git-url>` — existing behavior: shallow-clone to a temp directory, build, install.
- `spade install <local-path>` — **new**: skip the clone, treat the path as the source directory. Build happens **in place** (same working tree the user is editing), so tools like `cargo`, `go`, `uv`, and `bun` can reuse their incremental caches. Manifests and build artifacts are copied into `~/.spade/blocks/<collection>/<version>/` and registered just as they are for a git install.

Out of scope:
- No new `--path` flag. The first positional argument is the source specifier and its shape decides the mode.
- No changes to the artifact copy layout, registry schema, or cache behavior. Only the source-acquisition step changes.

---

## Phase 1: Source specifier detection

- [ ] In `cmd/install.go`, add a helper `isLocalSource(spec string) (absPath string, isLocal bool, err error)`.
  - Treat as a **git URL** when the spec matches `<scheme>://...` (with `scheme` one of `http`, `https`, `git`, `ssh`, `file`) **or** starts with `git@` (SCP-like git syntax).
  - Treat as a **local path** otherwise, and verify the resolved absolute path exists and is a directory. Return a clear error if the path does not exist or is a file rather than a directory.
  - Return the absolute, cleaned path via `filepath.Abs` + `filepath.Clean`. Resolve relative paths against the current working directory so `.` and `./foo` work.
- [ ] Preserve the current `file://` URL behavior: `file://` specs continue to go through the git clone path (so existing tests and the documented `file://$PWD` workflow remain valid).

### Tests (Phase 1)
- [ ] `cmd/install_test.go`: table-driven `TestIsLocalSource` covering:
  - `.` → local, resolves to CWD
  - `./sub` → local, resolves to absolute path
  - `/abs/path` → local when the directory exists; error when it doesn't
  - `https://example.com/foo.git` → not local
  - `git@github.com:org/repo.git` → not local
  - `file:///tmp/repo` → not local (still routed through git)
  - A file (not directory) → error

---

## Phase 2: Refactor `runInstall` around a "source directory"

- [ ] Extract the post-clone work in `runInstall` into `installFromSource(srcDir string, cleanupSrc bool) error`. Its responsibilities, in order:
  1. Detect language (`core.DetectLanguage`).
  2. Discover and load block manifests.
  3. Read collection name and version from the language manifest.
  4. Run the language-specific build in `srcDir`.
  5. Create `~/.spade/blocks/<collection>/<version>/` and copy manifests + artifacts.
  6. Compute the content hash and register every block.
  7. Clean up `srcDir` only when `cleanupSrc == true` (true for clones, false for user-provided local paths).
- [ ] Rewrite `runInstall(spec string) error` to:
  1. Call `isLocalSource(spec)`.
  2. If local, call `installFromSource(absPath, /*cleanupSrc=*/false)`.
  3. Otherwise, shallow-clone the URL into a temp directory and call `installFromSource(tmpDir, /*cleanupSrc=*/true)`.
- [ ] Keep the existing `os.RemoveAll(tmpDir)` guarded so we never delete a user-owned directory even on error paths (only the clone branch should `defer` cleanup).
- [ ] When running from a local path, print a different leading message (`Installing from local directory: <abs>`) so the operator can see at a glance that no clone occurred.

### Tests (Phase 2)
- [ ] `TestRunInstall_LocalPath_R`: create a temp directory with a valid R collection (`renv.lock`, `blocks/foo.yaml`, `R/foo.R`), call `runInstall(tempDir)`, and assert:
  - No clone happened (no `.git` created, temp dir still exists after install).
  - The block is registered in the registry.
  - Files were copied into `~/.spade/blocks/<collection>/<version>/`.
  - Re-invoking `runInstall(tempDir)` a second time is idempotent and does not duplicate the registry entry (matches existing behavior for the same version).
- [ ] `TestRunInstall_LocalPath_Dot`: use `t.Chdir(tempDir)` (Go 1.24+) or equivalent and call `runInstall(".")` — verify the same outcome as the absolute-path case.
- [ ] `TestRunInstall_LocalPath_NotADirectory`: pass a path to a regular file and assert the error message is clear and mentions the offending path.
- [ ] `TestRunInstall_LocalPath_Missing`: pass a path that doesn't exist and assert a clear error.
- [ ] Keep `TestRunInstall_LocalRepo` (existing `file://` flow) unchanged to prove the git path still works.

---

## Phase 3: Cobra command updates

- [ ] Update `installCmd` in `cmd/install.go`:
  - `Use: "install <git-url | path>"`
  - `Short: "Install a block collection from a git repository or local directory"`
  - `Long`: describe both modes and give one example of each, including `spade install .`.
- [ ] No flag changes; the positional argument shape decides the mode.

### Tests (Phase 3)
- [ ] `TestInstallCmdHelp`: assert the rendered `installCmd.Long` mentions both `git` and `.` (directory) examples, so the help stays in sync with behavior.

---

## Phase 4: Documentation updates

- [ ] Update `cli/README.md` under **`spade install`**: show `spade install .` as the canonical "install from current directory" example, and keep the git URL example.
- [ ] Update `../skills/spade/references/cli.md`:
  - Add `spade install .` / `spade install ./path` to the command summary block.
  - In the `## spade install` section, document that the argument may be a git URL *or* a local directory, and that `.` builds in place.
  - In the **End-to-end developer workflow** example, replace `spade install file://$PWD` with `spade install .`.
- [ ] Update `../spec/cli.md`:
  - Change the `spade install` spec to allow `<source>` being a git URL or a local path.
  - Note that local-path installs build in place and do not require the directory to be a git repository.

### Tests (Phase 4)
- [ ] No automated tests. Manual diff review of the three documents confirms each mentions the local-path form.

---

## Phase 5: Verification

- [ ] `go build ./...` succeeds.
- [ ] `go test ./...` passes.
- [ ] Manual smoke test (documented in the PR description, not run in CI):
  1. `spade setup`
  2. `mkdir /tmp/smoke && cd /tmp/smoke && spade init --language r`
  3. `spade add hello`
  4. `spade install .`
  5. Confirm the block appears in `~/.spade/blocks/smoke/0.1.0/` and in the registry.

---

## Summary of files touched

| File | Action | Description |
|------|--------|-------------|
| `cmd/install.go` | Modify | Source-specifier detection; split `runInstall` into `installFromSource`; updated cobra metadata. |
| `cmd/install_test.go` | Modify | New tests for local-path installs and `isLocalSource`. |
| `README.md` | Modify | Document `spade install .`. |
| `../skills/spade/references/cli.md` | Modify | Document `spade install .` in summary, section, and workflow example. |
| `../spec/cli.md` | Modify | Allow local path as the `spade install` source. |
