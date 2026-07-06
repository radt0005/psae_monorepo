// Package testutil provides shared fixtures for registry tests: most notably a
// real, buildable Go collection committed to a local git repo, so the full
// trust chain (clone → screen → build → sign → store → fetch → verify) can be
// exercised end to end without network access.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Collection describes a fixture collection repo (any language).
type Collection struct {
	RepoURL    string // file:// URL usable by `git clone`
	Dir        string
	CommitSHA  string
	Collection string
	Version    string
}

// GoCollection is retained as an alias for existing call sites.
type GoCollection = Collection

const fixtureMainGo = `package main

import (
	"fmt"
	"os"
)

// A minimal Spade Go collection binary with one subcommand.
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: greet")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "greet":
		fmt.Println("hello from the fixture collection")
	default:
		fmt.Fprintf(os.Stderr, "unknown block %q\n", os.Args[1])
		os.Exit(1)
	}
}
`

const fixtureBlockYAML = `id: hello.greet
version: 1.0.0
kind: standard
description: Emits a friendly greeting
inputs:
  name:
    type: string
    description: Who to greet
outputs:
  message:
    type: file
    format: text
    description: The greeting text
`

// NewGoCollectionRepo writes a buildable Go collection, commits it to a fresh
// git repo, and returns its coordinates. The collection is named "hello" at
// version 1.0.0.
func NewGoCollectionRepo(t *testing.T) GoCollection {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "go.mod"), "module hello\n\ngo 1.26\n")
	writeFile(t, filepath.Join(dir, "main.go"), fixtureMainGo)
	writeFile(t, filepath.Join(dir, "blocks", "greet.yaml"), fixtureBlockYAML)

	gitInit(t, dir)
	sha := gitCommitAll(t, dir, "initial collection")

	return GoCollection{
		RepoURL:    "file://" + dir,
		Dir:        dir,
		CommitSHA:  sha,
		Collection: "hello",
		Version:    "1.0.0",
	}
}

const fixtureMainRs = `use std::env;

// A minimal Spade Rust collection binary with one subcommand.
fn main() {
    let args: Vec<String> = env::args().collect();
    match args.get(1).map(|s| s.as_str()) {
        Some("greet") => println!("hello from the fixture collection"),
        other => {
            eprintln!("unknown block {:?}", other);
            std::process::exit(1);
        }
    }
}
`

// NewRustCollectionRepo writes a buildable Rust collection, commits it to a
// fresh git repo, and returns its coordinates. The crate (and thus the binary)
// is named "hello" so the produced binary matches the collection name.
func NewRustCollectionRepo(t *testing.T) Collection {
	t.Helper()
	dir := t.TempDir()

	cargoToml := "[package]\nname = \"hello\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[[bin]]\nname = \"hello\"\npath = \"src/main.rs\"\n"
	writeFile(t, filepath.Join(dir, "Cargo.toml"), cargoToml)
	writeFile(t, filepath.Join(dir, "src", "main.rs"), fixtureMainRs)
	writeFile(t, filepath.Join(dir, "blocks", "greet.yaml"), fixtureBlockYAML)

	gitInit(t, dir)
	sha := gitCommitAll(t, dir, "initial collection")

	return Collection{
		RepoURL:    "file://" + dir,
		Dir:        dir,
		CommitSHA:  sha,
		Collection: "hello",
		Version:    "1.0.0",
	}
}

const fixtureIndexTS = `// A minimal Spade TypeScript (Bun) collection with one subcommand.
const block = process.argv[2];
if (block === "greet") {
  console.log("hello from the fixture collection");
} else {
  console.error(` + "`unknown block ${block}`" + `);
  process.exit(1);
}
`

// NewBunCollectionRepo writes a buildable TypeScript (Bun) collection, commits
// it, and returns its coordinates. The collection is named "hello".
func NewBunCollectionRepo(t *testing.T) Collection {
	t.Helper()
	dir := t.TempDir()

	pkgJSON := "{\n  \"name\": \"hello\",\n  \"module\": \"index.ts\",\n  \"type\": \"module\"\n}\n"
	writeFile(t, filepath.Join(dir, "package.json"), pkgJSON)
	writeFile(t, filepath.Join(dir, "index.ts"), fixtureIndexTS)
	writeFile(t, filepath.Join(dir, "blocks", "greet.yaml"), fixtureBlockYAML)

	gitInit(t, dir)
	sha := gitCommitAll(t, dir, "initial collection")

	return Collection{
		RepoURL:    "file://" + dir,
		Dir:        dir,
		CommitSHA:  sha,
		Collection: "hello",
		Version:    "1.0.0",
	}
}

// --- Python (uv) fixture --------------------------------------------------

const fixturePyproject = `[project]
name = "hello"
version = "0.1.0"
requires-python = ">=3.10"
dependencies = []

[build-system]
requires = ["uv_build>=0.10,<0.11"]
build-backend = "uv_build"

[tool.uv.build-backend]
module-name = "hello"
`

const fixtureGreetPy = `def main():
    print("hello from the fixture collection")


if __name__ == "__main__":
    main()
`

// NewPythonCollectionRepo writes a buildable Python (uv) collection with one
// block module and no external dependencies, so `uv sync` runs offline. The
// package is "hello", block module "hello.greet".
func NewPythonCollectionRepo(t *testing.T) Collection {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "pyproject.toml"), fixturePyproject)
	writeFile(t, filepath.Join(dir, "src", "hello", "__init__.py"), "")
	writeFile(t, filepath.Join(dir, "src", "hello", "greet.py"), fixtureGreetPy)
	writeFile(t, filepath.Join(dir, "blocks", "greet.yaml"), fixtureBlockYAML)

	gitInit(t, dir)
	sha := gitCommitAll(t, dir, "initial collection")

	return Collection{
		RepoURL:    "file://" + dir,
		Dir:        dir,
		CommitSHA:  sha,
		Collection: "hello",
		Version:    "1.0.0",
	}
}

// --- R fixture ------------------------------------------------------------

// fixtureRenvLock is an empty (base-R-only) lockfile, matching how this repo's
// R collections declare deps via setup.R rather than renv.
const fixtureRenvLock = `{
  "R": { "Version": "4.6.0" },
  "Packages": {}
}
`

// fixtureSetupR installs nothing (base R only) but writes a marker into the
// artifact library, proving the builder pointed R_LIBS_USER inside the artifact.
const fixtureSetupR = `user_lib <- Sys.getenv("R_LIBS_USER")
dir.create(user_lib, recursive = TRUE, showWarnings = FALSE)
writeLines("ok", file.path(user_lib, "setup-marker"))
`

const fixtureGreetR = `cat("hello from the fixture collection\n")
`

// NewRCollectionRepo writes a buildable R collection using the setup.R
// convention with an empty renv.lock. setup.R uses only base R so the build
// runs offline; the block script prints a greeting.
func NewRCollectionRepo(t *testing.T) Collection {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "renv.lock"), fixtureRenvLock)
	writeFile(t, filepath.Join(dir, "setup.R"), fixtureSetupR)
	writeFile(t, filepath.Join(dir, "R", "greet.R"), fixtureGreetR)
	writeFile(t, filepath.Join(dir, "blocks", "greet.yaml"), fixtureBlockYAML)

	gitInit(t, dir)
	sha := gitCommitAll(t, dir, "initial collection")

	return Collection{
		RepoURL:    "file://" + dir,
		Dir:        dir,
		CommitSHA:  sha,
		Collection: "hello",
		Version:    "1.0.0",
	}
}

// fixtureDescriptionR declares one real dependency (jsonlite) via the pak
// DESCRIPTION path. jsonlite ships compiled code, so a round-trip test built on
// this fixture exercises relocation of a real .so, not just plain files.
const fixtureDescriptionR = `Package: hello.collection
Title: Spade R pak fixture collection
Version: 0.0.0
Imports:
    jsonlite
`

// fixtureGreetRPak loads the shipped dependency first (forcing its .so onto the
// search path from the relocated library) before greeting.
const fixtureGreetRPak = `library(jsonlite)
cat("hello from the fixture collection\n")
`

// NewRPakCollectionRepo writes a buildable R collection using the pak DESCRIPTION
// convention (no renv.lock, no setup.R). Unlike the setup.R fixture this one has a
// real dependency, so building it requires network access and pak to resolve
// jsonlite from a repo.
func NewRPakCollectionRepo(t *testing.T) Collection {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "DESCRIPTION"), fixtureDescriptionR)
	writeFile(t, filepath.Join(dir, "R", "greet.R"), fixtureGreetRPak)
	writeFile(t, filepath.Join(dir, "blocks", "greet.yaml"), fixtureBlockYAML)

	gitInit(t, dir)
	sha := gitCommitAll(t, dir, "initial collection")

	return Collection{
		RepoURL:    "file://" + dir,
		Dir:        dir,
		CommitSHA:  sha,
		Collection: "hello",
		Version:    "1.0.0",
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@spade.local")
	runGit(t, dir, "config", "user.name", "Spade Test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
}

func gitCommitAll(t *testing.T, dir, msg string) string {
	t.Helper()
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", msg)
	out := runGit(t, dir, "rev-parse", "HEAD")
	return strings.TrimSpace(out)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), out)
	return string(out)
}
