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

// GoCollection describes a fixture collection repo.
type GoCollection struct {
	RepoURL    string // file:// URL usable by `git clone`
	Dir        string
	CommitSHA  string
	Collection string
	Version    string
}

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
