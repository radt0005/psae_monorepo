package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunUpload_InvalidCollection(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Empty collection without blocks
	os.WriteFile("pyproject.toml", []byte(`[project]
name = "test"
`), 0644)
	os.MkdirAll("blocks", 0755)

	// Should fail validation (no blocks)
	// runUpload calls os.Exit, so we test ValidateCollection directly
	errs := ValidateCollection(".")
	if len(errs) == 0 {
		t.Error("expected validation errors for empty collection")
	}
}

func TestCreateArchive(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Set up files
	os.WriteFile("pyproject.toml", []byte(`[project]
name = "test"
version = "0.1.0"
`), 0644)
	os.MkdirAll(filepath.Join("src", "test"), 0755)
	os.WriteFile(filepath.Join("src", "test", "block.py"), []byte("pass\n"), 0644)
	os.MkdirAll("blocks", 0755)
	os.WriteFile(filepath.Join("blocks", "block.yaml"), []byte(`id: test.block
version: "0.1.0"
inputs: {}
outputs: {}
`), 0644)

	archiveName := "test-0.1.0.tar.gz"
	if err := createArchive(archiveName, ".", "python"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(archiveName)
	if err != nil {
		t.Fatalf("archive not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("archive is empty")
	}
}
