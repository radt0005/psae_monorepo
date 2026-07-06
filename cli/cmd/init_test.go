package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldRust(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	if err := scaffoldRust("test-collection"); err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, "Cargo.toml")
	assertFileExists(t, "src/lib.rs")
	assertDirExists(t, "blocks")
}

func TestScaffoldGo(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	if err := scaffoldGo("test-collection"); err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, "go.mod")
	assertFileExists(t, "main.go")
	assertDirExists(t, "blocks")
}

func TestScaffoldPython(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	if err := scaffoldPython("test-collection"); err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, "pyproject.toml")
	assertFileExists(t, filepath.Join("src", "test_collection", "__init__.py"))
	assertDirExists(t, "blocks")

	// The generated pyproject.toml must be a complete, buildable package so
	// `spade install` can produce a distribution that `uv run -m` can import.
	data, err := os.ReadFile("pyproject.toml")
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		`name = "test-collection"`,
		`requires-python = ">=3.12"`,
		`"pyyaml"`,
		"[build-system]",
		`build-backend = "uv_build"`,
		"[tool.uv.build-backend]",
		`module-name = "test_collection"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("pyproject.toml missing %q\n---\n%s", want, got)
		}
	}
}

func TestScaffoldTypeScript(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	if err := scaffoldTypeScript("test-collection"); err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, "package.json")
	assertDirExists(t, "src")
	assertDirExists(t, "blocks")
}

func TestScaffoldR(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	if err := scaffoldR("test-collection"); err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, "DESCRIPTION")
	assertDirExists(t, "R")
	assertDirExists(t, "blocks")
}

func TestInitLanguageFlag(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	initLanguage = "python"
	if err := runInit(); err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, "pyproject.toml")
	assertDirExists(t, "blocks")
}

func TestInitNoLanguage(t *testing.T) {
	initLanguage = ""
	if err := runInit(); err == nil {
		t.Error("expected error when no language specified")
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("file %s not found: %v", path, err)
		return
	}
	if info.IsDir() {
		t.Errorf("%s is a directory, expected file", path)
	}
}

func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("directory %s not found: %v", path, err)
		return
	}
	if !info.IsDir() {
		t.Errorf("%s is a file, expected directory", path)
	}
}
