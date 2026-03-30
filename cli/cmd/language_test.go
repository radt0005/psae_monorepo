package cmd

import (
	"core"
	"os"
	"path/filepath"
	"testing"
)

func TestReadCollectionName_Rust(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(`[package]
name = "my-rust-blocks"
version = "1.0.0"
`), 0644)

	name, err := ReadCollectionName(dir, core.CollectionLanguageRust)
	if err != nil {
		t.Fatal(err)
	}
	if name != "my-rust-blocks" {
		t.Errorf("got %q, want %q", name, "my-rust-blocks")
	}
}

func TestReadCollectionName_Python(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(`[project]
name = "my-python-blocks"
version = "2.0.0"
`), 0644)

	name, err := ReadCollectionName(dir, core.CollectionLanguagePython)
	if err != nil {
		t.Fatal(err)
	}
	if name != "my-python-blocks" {
		t.Errorf("got %q, want %q", name, "my-python-blocks")
	}
}

func TestReadCollectionName_TypeScript(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "my-ts-blocks", "version": "1.0.0"}`), 0644)

	name, err := ReadCollectionName(dir, core.CollectionLanguageTypeScript)
	if err != nil {
		t.Fatal(err)
	}
	if name != "my-ts-blocks" {
		t.Errorf("got %q, want %q", name, "my-ts-blocks")
	}
}

func TestReadCollectionName_Go(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/user/my-go-blocks\n\ngo 1.21\n"), 0644)

	name, err := ReadCollectionName(dir, core.CollectionLanguageGo)
	if err != nil {
		t.Fatal(err)
	}
	if name != "my-go-blocks" {
		t.Errorf("got %q, want %q", name, "my-go-blocks")
	}
}

func TestReadCollectionName_R(t *testing.T) {
	dir := t.TempDir()
	name, err := ReadCollectionName(dir, core.CollectionLanguageR)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Base(dir)
	if name != expected {
		t.Errorf("got %q, want %q", name, expected)
	}
}

func TestReadCollectionVersion_Rust(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(`[package]
name = "test"
version = "3.2.1"
`), 0644)

	version, err := ReadCollectionVersion(dir, core.CollectionLanguageRust)
	if err != nil {
		t.Fatal(err)
	}
	if version != "3.2.1" {
		t.Errorf("got %q, want %q", version, "3.2.1")
	}
}

func TestReadCollectionVersion_Python(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(`[project]
name = "test"
version = "1.2.3"
`), 0644)

	version, err := ReadCollectionVersion(dir, core.CollectionLanguagePython)
	if err != nil {
		t.Fatal(err)
	}
	if version != "1.2.3" {
		t.Errorf("got %q, want %q", version, "1.2.3")
	}
}

func TestReadCollectionVersion_TypeScript(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "test", "version": "4.5.6"}`), 0644)

	version, err := ReadCollectionVersion(dir, core.CollectionLanguageTypeScript)
	if err != nil {
		t.Fatal(err)
	}
	if version != "4.5.6" {
		t.Errorf("got %q, want %q", version, "4.5.6")
	}
}

func TestReadCollectionVersion_Go(t *testing.T) {
	version, err := ReadCollectionVersion(t.TempDir(), core.CollectionLanguageGo)
	if err != nil {
		t.Fatal(err)
	}
	if version != "0.1.0" {
		t.Errorf("got %q, want %q", version, "0.1.0")
	}
}

func TestReadCollectionVersion_R(t *testing.T) {
	version, err := ReadCollectionVersion(t.TempDir(), core.CollectionLanguageR)
	if err != nil {
		t.Fatal(err)
	}
	if version != "0.1.0" {
		t.Errorf("got %q, want %q", version, "0.1.0")
	}
}
