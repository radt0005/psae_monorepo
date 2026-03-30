package cmd

import (
	"core"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCopyCollection(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source structure
	os.MkdirAll(filepath.Join(srcDir, "blocks"), 0755)
	os.WriteFile(filepath.Join(srcDir, "blocks", "test.yaml"), []byte("id: test\n"), 0644)
	os.MkdirAll(filepath.Join(srcDir, "R"), 0755)
	os.WriteFile(filepath.Join(srcDir, "R", "test.R"), []byte("# test\n"), 0644)

	if err := copyCollection(srcDir, dstDir, core.CollectionLanguageR, "test"); err != nil {
		t.Fatal(err)
	}

	assertFileExists(t, filepath.Join(dstDir, "blocks", "test.yaml"))
	assertFileExists(t, filepath.Join(dstDir, "R", "test.R"))
}

func TestRunInstall_InvalidURL(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)

	// Set up spade directory
	os.MkdirAll(filepath.Join(dir, "blocks"), 0755)

	err := runInstall("https://invalid-url-that-does-not-exist.example.com/repo.git")
	if err == nil {
		t.Error("expected error for invalid git URL")
	}
}

func TestRunInstall_LocalRepo(t *testing.T) {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	spadeDir := t.TempDir()
	t.Setenv("SPADE_DIR", spadeDir)
	os.MkdirAll(filepath.Join(spadeDir, "blocks"), 0755)

	// Create a local git repo with a valid R collection
	repoDir := t.TempDir()
	os.MkdirAll(filepath.Join(repoDir, "blocks"), 0755)
	os.MkdirAll(filepath.Join(repoDir, "R"), 0755)
	os.WriteFile(filepath.Join(repoDir, "blocks", "test.yaml"), []byte(`id: test-collection.test
version: "0.1.0"
kind: standard
inputs: {}
outputs: {}
`), 0644)
	os.WriteFile(filepath.Join(repoDir, "R", "test.R"), []byte("# test block\n"), 0644)

	// Initialize git repo
	gitInit := exec.Command("git", "init")
	gitInit.Dir = repoDir
	if err := gitInit.Run(); err != nil {
		t.Fatal(err)
	}

	gitConfig1 := exec.Command("git", "config", "user.email", "test@test.com")
	gitConfig1.Dir = repoDir
	gitConfig1.Run()

	gitConfig2 := exec.Command("git", "config", "user.name", "Test")
	gitConfig2.Dir = repoDir
	gitConfig2.Run()

	gitAdd := exec.Command("git", "add", ".")
	gitAdd.Dir = repoDir
	if err := gitAdd.Run(); err != nil {
		t.Fatal(err)
	}

	gitCommit := exec.Command("git", "commit", "-m", "initial")
	gitCommit.Dir = repoDir
	if err := gitCommit.Run(); err != nil {
		t.Fatal(err)
	}

	// Install from local file:// URL
	fileURL := "file://" + repoDir
	if err := runInstall(fileURL); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Verify blocks were installed
	registry, err := core.OpenRegistry(filepath.Join(spadeDir, "registry.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	blocks, err := registry.ListBlocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) == 0 {
		t.Error("expected at least one block in registry after install")
	}
}
