package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateCollection_Valid(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Set up a valid Python collection
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
    format: GeoTIFF
outputs:
  result:
    type: file
    format: GeoTIFF
`), 0644)

	errs := ValidateCollection(".")
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("unexpected error: %v", e)
		}
	}
}

func TestValidateCollection_MissingFields(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.WriteFile("pyproject.toml", []byte(`[project]
name = "test"
`), 0644)
	os.MkdirAll("blocks", 0755)
	// Missing id and version
	os.WriteFile(filepath.Join("blocks", "bad.yaml"), []byte(`inputs:
  data:
    type: file
outputs:
  result:
    type: file
`), 0644)

	errs := ValidateCollection(".")
	if len(errs) == 0 {
		t.Error("expected validation errors for missing fields")
	}

	hasIDError := false
	hasVersionError := false
	for _, e := range errs {
		if searchString(e.Error(), "id") {
			hasIDError = true
		}
		if searchString(e.Error(), "version") {
			hasVersionError = true
		}
	}
	if !hasIDError {
		t.Error("expected error about missing id")
	}
	if !hasVersionError {
		t.Error("expected error about missing version")
	}
}

func TestValidateCollection_InvalidTypes(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.WriteFile("pyproject.toml", []byte(`[project]
name = "test"
`), 0644)
	os.MkdirAll(filepath.Join("src", "test"), 0755)
	os.WriteFile(filepath.Join("src", "test", "bad.py"), []byte("pass\n"), 0644)
	os.MkdirAll("blocks", 0755)
	os.WriteFile(filepath.Join("blocks", "bad.yaml"), []byte(`id: test.bad
version: "0.1.0"
inputs:
  data:
    type: invalid_type
outputs:
  result:
    type: also_invalid
`), 0644)

	errs := ValidateCollection(".")
	if len(errs) == 0 {
		t.Error("expected validation errors for invalid types")
	}
}

func TestValidateCollection_BadIDFormat(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.WriteFile("pyproject.toml", []byte(`[project]
name = "test"
`), 0644)
	os.MkdirAll(filepath.Join("src", "test"), 0755)
	os.WriteFile(filepath.Join("src", "test", "bad.py"), []byte("pass\n"), 0644)
	os.MkdirAll("blocks", 0755)
	os.WriteFile(filepath.Join("blocks", "bad.yaml"), []byte(`id: nodot
version: "0.1.0"
inputs: {}
outputs: {}
`), 0644)

	errs := ValidateCollection(".")
	found := false
	for _, e := range errs {
		if searchString(e.Error(), "convention") {
			found = true
		}
	}
	if !found {
		t.Error("expected error about ID convention")
	}
}

func TestValidateCollection_NoBlocks(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.WriteFile("pyproject.toml", []byte(`[project]
name = "test"
`), 0644)
	os.MkdirAll("blocks", 0755)

	errs := ValidateCollection(".")
	if len(errs) == 0 {
		t.Error("expected error for no blocks")
	}
}
