package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpadeDir_Default(t *testing.T) {
	os.Unsetenv("SPADE_DIR")
	dir := SpadeDir()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".spade")
	if dir != expected {
		t.Errorf("SpadeDir() = %q, want %q", dir, expected)
	}
}

func TestSpadeDir_Override(t *testing.T) {
	t.Setenv("SPADE_DIR", "/tmp/test-spade")
	dir := SpadeDir()
	if dir != "/tmp/test-spade" {
		t.Errorf("SpadeDir() = %q, want %q", dir, "/tmp/test-spade")
	}
}

func TestBlocksDir(t *testing.T) {
	t.Setenv("SPADE_DIR", "/tmp/test-spade")
	if got := BlocksDir(); got != "/tmp/test-spade/blocks" {
		t.Errorf("BlocksDir() = %q, want %q", got, "/tmp/test-spade/blocks")
	}
}

func TestCacheDir(t *testing.T) {
	t.Setenv("SPADE_DIR", "/tmp/test-spade")
	if got := CacheDir(); got != "/tmp/test-spade/cache" {
		t.Errorf("CacheDir() = %q, want %q", got, "/tmp/test-spade/cache")
	}
}

func TestPipelinesDir(t *testing.T) {
	t.Setenv("SPADE_DIR", "/tmp/test-spade")
	if got := PipelinesDir(); got != "/tmp/test-spade/pipelines" {
		t.Errorf("PipelinesDir() = %q, want %q", got, "/tmp/test-spade/pipelines")
	}
}

func TestRegistryPath(t *testing.T) {
	t.Setenv("SPADE_DIR", "/tmp/test-spade")
	if got := RegistryPath(); got != "/tmp/test-spade/registry.db" {
		t.Errorf("RegistryPath() = %q, want %q", got, "/tmp/test-spade/registry.db")
	}
}
