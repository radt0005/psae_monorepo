package cmd

import (
	"core"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// newRCollection creates a named R-language collection subdirectory so that
// filepath.Base(srcDir) yields a stable, predictable collection name.
func newRCollection(t *testing.T, collectionName string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, collectionName)
	if err := os.MkdirAll(filepath.Join(dir, "blocks"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "R"), 0755); err != nil {
		t.Fatal(err)
	}
	manifest := "id: " + collectionName + ".hello\nversion: \"0.1.0\"\nkind: standard\nentrypoint: hello\ninputs: {}\noutputs: {}\n"
	if err := os.WriteFile(filepath.Join(dir, "blocks", "hello.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "R", "hello.R"), []byte("# hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRunInstall_LocalPath(t *testing.T) {
	spadeDir := t.TempDir()
	t.Setenv("SPADE_DIR", spadeDir)
	if err := os.MkdirAll(filepath.Join(spadeDir, "blocks"), 0755); err != nil {
		t.Fatal(err)
	}

	srcDir := newRCollection(t, "localtest")

	if err := runInstall(srcDir); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Source directory must still exist — we must NOT remove user-owned dirs.
	if _, err := os.Stat(srcDir); err != nil {
		t.Errorf("source dir was removed: %v", err)
	}

	// No .git directory should have been created — no clone happened.
	if _, err := os.Stat(filepath.Join(srcDir, ".git")); !os.IsNotExist(err) {
		t.Errorf("expected no .git directory in source, got err=%v", err)
	}

	// Verify install artifacts.
	installed := filepath.Join(spadeDir, "blocks", "localtest", "0.1.0")
	assertFileExists(t, filepath.Join(installed, "blocks", "hello.yaml"))
	assertFileExists(t, filepath.Join(installed, "R", "hello.R"))

	// Verify registry entry.
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
		t.Error("expected at least one block in registry after local install")
	}
}

func TestRunInstall_LocalPath_Dot(t *testing.T) {
	spadeDir := t.TempDir()
	t.Setenv("SPADE_DIR", spadeDir)
	if err := os.MkdirAll(filepath.Join(spadeDir, "blocks"), 0755); err != nil {
		t.Fatal(err)
	}

	srcDir := newRCollection(t, "dottest")
	t.Chdir(srcDir)

	if err := runInstall("."); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	installed := filepath.Join(spadeDir, "blocks", "dottest", "0.1.0")
	assertFileExists(t, filepath.Join(installed, "blocks", "hello.yaml"))
}

func TestRunInstall_LocalPath_Idempotent(t *testing.T) {
	spadeDir := t.TempDir()
	t.Setenv("SPADE_DIR", spadeDir)
	if err := os.MkdirAll(filepath.Join(spadeDir, "blocks"), 0755); err != nil {
		t.Fatal(err)
	}

	srcDir := newRCollection(t, "idem")

	if err := runInstall(srcDir); err != nil {
		t.Fatalf("first install failed: %v", err)
	}
	if err := runInstall(srcDir); err != nil {
		t.Fatalf("second install failed: %v", err)
	}

	registry, err := core.OpenRegistry(filepath.Join(spadeDir, "registry.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()
	blocks, err := registry.ListBlocks()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, b := range blocks {
		if b.CollectionName == "idem" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 block for collection 'idem' after two installs, got %d", count)
	}
}

func TestRunInstall_LocalPath_NotADirectory(t *testing.T) {
	spadeDir := t.TempDir()
	t.Setenv("SPADE_DIR", spadeDir)

	tmpFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(tmpFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	err := runInstall(tmpFile)
	if err == nil {
		t.Fatal("expected error for non-directory local path")
	}
}

func TestRunInstall_LocalPath_Missing(t *testing.T) {
	spadeDir := t.TempDir()
	t.Setenv("SPADE_DIR", spadeDir)

	missing := filepath.Join(t.TempDir(), "nope")
	err := runInstall(missing)
	if err == nil {
		t.Fatal("expected error for missing local path")
	}
}

func TestInstallCmdHelp(t *testing.T) {
	long := installCmd.Long
	for _, needle := range []string{"git", "spade install .", "local directory"} {
		if !strings.Contains(long, needle) {
			t.Errorf("installCmd.Long missing %q; got:\n%s", needle, long)
		}
	}
	if !strings.Contains(installCmd.Use, "path") {
		t.Errorf("installCmd.Use missing 'path'; got %q", installCmd.Use)
	}
}

func TestIsLocalSource(t *testing.T) {
	existingDir := t.TempDir()
	existingFile := filepath.Join(existingDir, "afile")
	if err := os.WriteFile(existingFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		spec    string
		wantLoc bool
		wantErr bool
		wantAbs string
	}{
		{name: "https URL", spec: "https://example.com/foo.git", wantLoc: false},
		{name: "http URL", spec: "http://example.com/foo.git", wantLoc: false},
		{name: "git scheme", spec: "git://example.com/foo.git", wantLoc: false},
		{name: "ssh scheme", spec: "ssh://git@example.com/foo.git", wantLoc: false},
		{name: "file scheme", spec: "file:///tmp/repo", wantLoc: false},
		{name: "git@ SCP syntax", spec: "git@github.com:org/repo.git", wantLoc: false},
		{name: "existing abs dir", spec: existingDir, wantLoc: true, wantAbs: existingDir},
		{name: "missing path", spec: filepath.Join(existingDir, "does-not-exist"), wantLoc: true, wantErr: true},
		{name: "file not dir", spec: existingFile, wantLoc: true, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			abs, isLocal, err := isLocalSource(tc.spec)
			if isLocal != tc.wantLoc {
				t.Errorf("isLocal = %v, want %v (err=%v)", isLocal, tc.wantLoc, err)
			}
			if tc.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantAbs != "" && abs != tc.wantAbs {
				t.Errorf("abs = %q, want %q", abs, tc.wantAbs)
			}
		})
	}
}

func TestIsLocalSource_Dot(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	abs, isLocal, err := isLocalSource(".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isLocal {
		t.Fatalf("expected local, got git URL")
	}
	wantAbs, _ := filepath.Abs(dir)
	if abs != wantAbs {
		t.Errorf("abs = %q, want %q", abs, wantAbs)
	}
}

func TestIsLocalSource_RelativeSub(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	abs, isLocal, err := isLocalSource("./sub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isLocal {
		t.Fatalf("expected local, got git URL")
	}
	wantAbs, _ := filepath.Abs(sub)
	if abs != wantAbs {
		t.Errorf("abs = %q, want %q", abs, wantAbs)
	}
}
