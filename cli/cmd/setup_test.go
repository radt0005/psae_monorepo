package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunSetup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)

	if err := runSetup(); err != nil {
		t.Fatal(err)
	}

	// Check directories were created
	for _, subdir := range []string{"blocks", "cache", "pipelines"} {
		path := filepath.Join(dir, subdir)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("directory %s not created: %v", subdir, err)
		} else if !info.IsDir() {
			t.Errorf("%s is not a directory", subdir)
		}
	}

	// Check registry was created
	regPath := filepath.Join(dir, "registry.db")
	if _, err := os.Stat(regPath); err != nil {
		t.Errorf("registry not created: %v", err)
	}
}

func TestRunSetup_Idempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)

	if err := runSetup(); err != nil {
		t.Fatal(err)
	}
	// Running again should not error
	if err := runSetup(); err != nil {
		t.Fatalf("second setup failed: %v", err)
	}
}
