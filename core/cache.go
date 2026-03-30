package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"gopkg.in/yaml.v3"
)

// ComputeCacheKey computes a deterministic cache key from the block ID, version,
// input content hashes, and serialized params.
func ComputeCacheKey(blockID string, blockVersion string, inputHashes map[string]string, params map[string]any) (string, error) {
	h := sha256.New()

	// Block identity
	h.Write([]byte(blockID))
	h.Write([]byte(blockVersion))

	// Input hashes in deterministic order
	inputNames := make([]string, 0, len(inputHashes))
	for name := range inputHashes {
		inputNames = append(inputNames, name)
	}
	sort.Strings(inputNames)
	for _, name := range inputNames {
		h.Write([]byte(name))
		h.Write([]byte(inputHashes[name]))
	}

	// Params serialized deterministically
	if params != nil {
		paramsData, err := yaml.Marshal(params)
		if err != nil {
			return "", fmt.Errorf("marshaling params for cache key: %w", err)
		}
		h.Write(paramsData)
	}

	// Runtime hash
	runtimeHash, err := ComputeRuntimeHash()
	if err != nil {
		return "", err
	}
	h.Write([]byte(runtimeHash))

	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeRuntimeHash hashes the relevant runtime environment.
func ComputeRuntimeHash() (string, error) {
	h := sha256.New()
	h.Write([]byte(runtime.Version()))
	h.Write([]byte(runtime.GOOS))
	h.Write([]byte(runtime.GOARCH))
	return hex.EncodeToString(h.Sum(nil)), nil
}

// CacheLookup checks if cached outputs exist for the given key.
func CacheLookup(cacheKey string, cacheDir string) (string, bool) {
	cachePath := filepath.Join(cacheDir, cacheKey)
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, true
	}
	return "", false
}

// CacheStore copies the block's outputs to the cache directory keyed by the cache key.
func CacheStore(cacheKey string, outputsDir string, cacheDir string) error {
	cachePath := filepath.Join(cacheDir, cacheKey)
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	return copyDir(outputsDir, cachePath)
}

// CacheRestore restores cached outputs into a block's working directory.
func CacheRestore(cacheKey string, targetDir string, cacheDir string) error {
	cachePath := filepath.Join(cacheDir, cacheKey)
	return copyDir(cachePath, targetDir)
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		return copyFile(path, target)
	})
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
