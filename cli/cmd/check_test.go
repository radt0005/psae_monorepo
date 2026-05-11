package cmd

import (
	"bytes"
	"core"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCheckCollection_Valid(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Valid Python collection
	os.WriteFile("pyproject.toml", []byte(`[project]
name = "test-blocks"
version = "0.1.0"
`), 0644)
	os.MkdirAll(filepath.Join("src", "test_blocks"), 0755)
	os.WriteFile(filepath.Join("src", "test_blocks", "myblock.py"), []byte("pass\n"), 0644)
	os.MkdirAll("blocks", 0755)
	os.WriteFile(filepath.Join("blocks", "myblock.yaml"), []byte(`id: test-blocks.myblock
version: "0.1.0"
kind: standard
inputs:
  data:
    type: file
outputs:
  result:
    type: file
`), 0644)

	if err := runCheckCollection(); err != nil {
		t.Fatal(err)
	}
}

func TestRunCheckPipeline_BlockNotInRegistry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)

	// Setup registry
	os.MkdirAll(dir, 0755)
	registry, err := core.OpenRegistry(filepath.Join(dir, "registry.db"))
	if err != nil {
		t.Fatal(err)
	}
	registry.Close()

	// Create a pipeline referencing a block not in registry
	pipelinePath := filepath.Join(dir, "pipeline.yaml")
	os.WriteFile(pipelinePath, []byte(`id: 019cf4bc-0000-7000-0000-000000000000
name: test-pipeline
version: "1.0"
blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: nonexistent.block
    inputs: []
    args: {}
`), 0644)

	err = runCheckPipeline(pipelinePath)
	if err == nil {
		t.Error("expected error for non-existent block in registry")
	}
}

func TestRunCheckPipeline_ShortCodeGeneratesLockfile(t *testing.T) {
	// The pipeline references blocks not in the registry, so
	// runCheckPipeline will fail after lockfile generation.  The test
	// asserts that the lockfile was written before the failure.
	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)
	os.MkdirAll(dir, 0755)
	registry, err := core.OpenRegistry(filepath.Join(dir, "registry.db"))
	if err != nil {
		t.Fatal(err)
	}
	registry.Close()

	pipelinePath := filepath.Join(dir, "pipeline.yaml")
	os.WriteFile(pipelinePath, []byte(`name: test
version: "1.0"
blocks:
  - id: "@source"
    name: data.x
    inputs: []
    args: {}
  - id: "@out"
    name: data.y
    inputs:
      - "@source"
    args: {}
`), 0644)

	// runCheckPipeline will return an error because the blocks aren't
	// in the registry — but the lockfile should still be created.
	_ = runCheckPipeline(pipelinePath)

	lockPath := core.LockfilePathFor(pipelinePath)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lockfile at %s, got: %v", lockPath, err)
	}
	lock, err := core.LoadLockfile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(lock.Bindings))
	}
}

func TestRunCheckPipeline_ShortCodeIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)
	os.MkdirAll(dir, 0755)
	registry, _ := core.OpenRegistry(filepath.Join(dir, "registry.db"))
	registry.Close()

	pipelinePath := filepath.Join(dir, "pipeline.yaml")
	os.WriteFile(pipelinePath, []byte(`name: test
version: "1.0"
blocks:
  - id: "@a"
    name: data.x
    inputs: []
    args: {}
`), 0644)

	_ = runCheckPipeline(pipelinePath)
	lockPath := core.LockfilePathFor(pipelinePath)
	first, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}

	_ = runCheckPipeline(pipelinePath)
	second, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("lockfile changed on idempotent rerun:\n--- first ---\n%s--- second ---\n%s", first, second)
	}
}

func TestRunCheckPipeline_InvalidLockfile(t *testing.T) {
	// Use a subprocess so os.Exit(1) doesn't terminate the test process.
	if os.Getenv("SPADE_CHECK_INVALID_LOCK_CHILD") == "1" {
		dir := os.Getenv("SPADE_CHECK_INVALID_LOCK_DIR")
		_ = runCheckPipeline(filepath.Join(dir, "pipeline.yaml"))
		return
	}

	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)
	os.MkdirAll(dir, 0755)
	registry, _ := core.OpenRegistry(filepath.Join(dir, "registry.db"))
	registry.Close()

	pipelinePath := filepath.Join(dir, "pipeline.yaml")
	os.WriteFile(pipelinePath, []byte(`name: test
version: "1.0"
blocks:
  - id: "@a"
    name: data.x
    inputs: []
    args: {}
`), 0644)
	// Pre-populate a malformed lockfile.
	os.WriteFile(core.LockfilePathFor(pipelinePath), []byte(`bindings:
  "@a": not-a-uuid
`), 0644)

	cmd := exec.Command(os.Args[0], "-test.run=TestRunCheckPipeline_InvalidLockfile")
	cmd.Env = append(os.Environ(),
		"SPADE_CHECK_INVALID_LOCK_CHILD=1",
		"SPADE_DIR="+dir,
		"SPADE_CHECK_INVALID_LOCK_DIR="+dir,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected subprocess to exit non-zero")
	}
	out := stderr.String()
	if !strings.Contains(out, "invalid lockfile") {
		t.Errorf("expected error to mention 'invalid lockfile', got: %s", out)
	}
	if !strings.Contains(out, "delete") {
		t.Errorf("expected error to mention deleting the lockfile, got: %s", out)
	}
}
