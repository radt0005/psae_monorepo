package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunAdd_Python(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Set up a Python collection
	os.WriteFile("pyproject.toml", []byte(`[project]
name = "test-blocks"
version = "0.1.0"
`), 0644)
	os.MkdirAll(filepath.Join("src", "test_blocks"), 0755)
	os.WriteFile(filepath.Join("src", "test_blocks", "__init__.py"), []byte(""), 0644)

	if err := runAdd("myblock"); err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, filepath.Join("blocks", "myblock.yaml"))
	assertFileExists(t, filepath.Join("src", "test_blocks", "myblock.py"))
}

func TestRunAdd_Rust(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.WriteFile("Cargo.toml", []byte(`[package]
name = "test"
version = "0.1.0"
`), 0644)
	os.MkdirAll("src", 0755)

	if err := runAdd("myblock"); err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, filepath.Join("blocks", "myblock.yaml"))
	assertFileExists(t, filepath.Join("src", "myblock.rs"))
}

func TestRunAdd_Go(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.WriteFile("go.mod", []byte("module test\n\ngo 1.21\n"), 0644)

	if err := runAdd("myblock"); err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, filepath.Join("blocks", "myblock.yaml"))
	assertFileExists(t, "myblock.go")
}

func TestRunAdd_R(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// R detection: no marker files → defaults to R
	os.MkdirAll("R", 0755)

	if err := runAdd("myblock"); err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, filepath.Join("blocks", "myblock.yaml"))
	assertFileExists(t, filepath.Join("R", "myblock.R"))
}

func TestRunAdd_TypeScript(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.WriteFile("package.json", []byte(`{"name": "test", "version": "0.1.0"}`), 0644)
	os.MkdirAll("src", 0755)

	if err := runAdd("myblock"); err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, filepath.Join("blocks", "myblock.yaml"))
	assertFileExists(t, filepath.Join("src", "myblock.ts"))
}

func TestRunAdd_BlockIDConvention(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Create a directory named "mypackage" so the block ID will be "mypackage.myblock"
	os.WriteFile("go.mod", []byte("module test\n\ngo 1.21\n"), 0644)

	if err := runAdd("myblock"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join("blocks", "myblock.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	// The block ID should contain a dot
	content := string(data)
	if !contains(content, ".myblock") {
		t.Errorf("block manifest should contain '.myblock' in id, got:\n%s", content)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
