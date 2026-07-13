+++
title = "spade publish"
description = "Submit a block collection to the cloud registry for screening, build, and distribution."
weight = 7
+++

The `spade publish` command submits the current block collection to the cloud registry for screening, build, signing, and distribution. It is run from the root of a block collection directory, on a commit that has already been pushed to a git remote.

`spade publish` does **not** upload an artifact and does not build anything locally. It submits a reference — `(repo_url, commit_sha, collection_name, version)` — and the registry does the rest: it clones your repository at that exact commit, screens the source, builds the artifact in the bundler image, signs it, and stores it. This replaces the earlier `spade upload` command, which packaged and shipped a local tarball.

## Usage

```bash
spade publish
```

No arguments are required. The command operates on the collection in the current working directory, which must be a git repository with a configured remote.

## Preconditions

Before submitting anything, `spade publish` requires:

1. **A `spade login` session.** The submission is authenticated as you. See [`spade login`](/cli/login/).
2. **A clean working tree.** No uncommitted changes.
3. **A pushed `HEAD`.** The current commit must be reachable on the configured remote.

If the working tree is dirty or `HEAD` hasn't been pushed, `spade publish` refuses to submit. This isn't just tidiness — it's the foundation of the registry's trust chain. The registry screens source and then builds from that *same* commit SHA, so the artifact it signs is provably the result of running the build pipeline against the exact code that was screened. If the CLI allowed publishing local, unpushed changes, the screened source and the deployed source could diverge, and the screening signal would be meaningless.

## What it does

### 1. Validate the collection

Runs the same checks as [`spade check`](/cli/check/) against the working tree. If any errors are found, the command prints them and exits with status 1 without submitting anything.

### 2. Verify the tree is clean and pushed

Confirms there are no uncommitted changes and that `HEAD` is reachable on the configured remote. If either check fails, the command exits with an explanation and does not contact the registry.

### 3. Resolve collection name and version

The collection name and version are read from the language's own manifest (`Cargo.toml`, `pyproject.toml`, `go.mod`, `package.json`, or the R project layout) — the same resolution `spade install` and `spade check` use.

### 4. Submit to the registry

Using the session created by `spade login`, the CLI submits `(repo_url, commit_sha, collection_name, version)` to the registry as a publish request.

### 5. Track progress

The command prints the registry URL where you can follow the submission through screening, build, and signing.

## Lifecycle after submission

Once submitted, the collection version moves through a sequence of states on the registry side. `spade publish` itself only performs the submission (step 4 above) — everything after that happens asynchronously on the registry:

| State | Meaning |
|-------|---------|
| `submitted` | Publish request received, waiting in the screening queue |
| `screening` | Screening pipeline is running against the cloned source |
| `screened` | Screening passed; awaiting human approval (if required) or automatic build |
| `building` | The registry is building the artifact in the bundler image |
| `available` | Build, sign, and store are complete — workers may install and execute this version |
| `failed` | Screening, build, or sign step failed; check the registry URL for logs |

The happy path is `submitted` → `screening` → `screened` → `building` → `available`. Build only ever runs after screening passes, which is what lets the registry — not the developer — control the bytes that get signed and shipped.

Check on a submission's progress at the URL printed by `spade publish`, or via the registry's status endpoint for the collection and version.

## Example

```bash
cd my-collection
git push origin main
spade publish
```

Output on success:

```
Collection is valid. 4 block(s) checked.
Working tree is clean, HEAD is pushed (a1b2c3d on origin/main).
Submitted my-collection v1.2.0 for publishing.
Track progress at https://registry.spade.dev/collections/my-collection/1.2.0
```

Output when the tree isn't ready to publish:

```
spade publish: refusing to submit
  working tree has uncommitted changes -- commit or stash them first
```

```
spade publish: refusing to submit
  HEAD (a1b2c3d) is not reachable on remote "origin" -- push it first
```

## See also

- [`spade login`](/cli/login/) for authenticating before you publish
- [`spade check`](/cli/check/) for running validation independently
- [`spade install`](/cli/install/) for fetching a published (or locally built) collection
- [Block Collections](/concepts/collections/) for the publishing and registry overview
