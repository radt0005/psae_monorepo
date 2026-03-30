package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeCacheKeySameInputs(t *testing.T) {
	inputHashes := map[string]string{
		"source": "abc123",
		"target": "def456",
	}
	params := map[string]any{
		"method": "bilinear",
		"scale":  2.0,
	}

	key1, err := ComputeCacheKey("test.block", "1.0.0", inputHashes, params)
	if err != nil {
		t.Fatalf("ComputeCacheKey failed: %v", err)
	}

	key2, err := ComputeCacheKey("test.block", "1.0.0", inputHashes, params)
	if err != nil {
		t.Fatalf("ComputeCacheKey failed: %v", err)
	}

	if key1 != key2 {
		t.Error("same inputs should produce same cache key")
	}
}

func TestComputeCacheKeyDifferentInputs(t *testing.T) {
	params := map[string]any{"method": "bilinear"}

	key1, _ := ComputeCacheKey("test.block", "1.0.0",
		map[string]string{"source": "abc123"}, params)

	key2, _ := ComputeCacheKey("test.block", "1.0.0",
		map[string]string{"source": "different"}, params)

	if key1 == key2 {
		t.Error("different inputs should produce different cache key")
	}

	// Different block version
	key3, _ := ComputeCacheKey("test.block", "2.0.0",
		map[string]string{"source": "abc123"}, params)

	if key1 == key3 {
		t.Error("different version should produce different cache key")
	}

	// Different params
	key4, _ := ComputeCacheKey("test.block", "1.0.0",
		map[string]string{"source": "abc123"},
		map[string]any{"method": "nearest"})

	if key1 == key4 {
		t.Error("different params should produce different cache key")
	}
}

func TestCacheStoreAndLookup(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	outputsDir := filepath.Join(dir, "outputs")

	// Create mock outputs
	os.MkdirAll(filepath.Join(outputsDir, "raster"), 0755)
	os.WriteFile(filepath.Join(outputsDir, "raster", "data.tif"), []byte("mock tif"), 0644)

	cacheKey := "test-cache-key-abc123"

	// Store
	if err := CacheStore(cacheKey, outputsDir, cacheDir); err != nil {
		t.Fatalf("CacheStore failed: %v", err)
	}

	// Lookup
	path, hit := CacheLookup(cacheKey, cacheDir)
	if !hit {
		t.Fatal("expected cache hit")
	}
	if path == "" {
		t.Fatal("expected non-empty cache path")
	}

	// Verify cached file exists
	cachedFile := filepath.Join(path, "raster", "data.tif")
	data, err := os.ReadFile(cachedFile)
	if err != nil {
		t.Fatalf("reading cached file: %v", err)
	}
	if string(data) != "mock tif" {
		t.Errorf("expected 'mock tif', got %q", string(data))
	}
}

func TestCacheRestore(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	outputsDir := filepath.Join(dir, "outputs")
	targetDir := filepath.Join(dir, "target")

	// Create mock outputs
	os.MkdirAll(filepath.Join(outputsDir, "result"), 0755)
	os.WriteFile(filepath.Join(outputsDir, "result", "output.json"), []byte(`{"status": "ok"}`), 0644)

	cacheKey := "restore-key-abc123"

	// Store first
	CacheStore(cacheKey, outputsDir, cacheDir)

	// Restore
	os.MkdirAll(targetDir, 0755)
	if err := CacheRestore(cacheKey, targetDir, cacheDir); err != nil {
		t.Fatalf("CacheRestore failed: %v", err)
	}

	// Verify restored file
	restoredFile := filepath.Join(targetDir, "result", "output.json")
	data, err := os.ReadFile(restoredFile)
	if err != nil {
		t.Fatalf("reading restored file: %v", err)
	}
	if string(data) != `{"status": "ok"}` {
		t.Errorf("unexpected restored content: %q", string(data))
	}
}

func TestCacheMiss(t *testing.T) {
	dir := t.TempDir()

	_, hit := CacheLookup("nonexistent-key", dir)
	if hit {
		t.Error("expected cache miss for nonexistent key")
	}
}

func TestComputeRuntimeHash(t *testing.T) {
	hash1, err := ComputeRuntimeHash()
	if err != nil {
		t.Fatalf("ComputeRuntimeHash failed: %v", err)
	}
	if len(hash1) != 64 {
		t.Errorf("expected 64-char hash, got %d chars", len(hash1))
	}

	// Same runtime should produce same hash
	hash2, _ := ComputeRuntimeHash()
	if hash1 != hash2 {
		t.Error("same runtime should produce same hash")
	}
}
