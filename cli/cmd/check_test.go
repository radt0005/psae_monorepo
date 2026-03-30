package cmd

import (
	"core"
	"os"
	"path/filepath"
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
