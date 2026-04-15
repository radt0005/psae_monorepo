package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// CreateBlockDirectory creates the full directory structure for a block invocation:
// the main directory, plus inputs/, outputs/, and logs/ subdirectories.
func CreateBlockDirectory(id uuid.UUID, workdir string) error {
	base := filepath.Join(workdir, id.String())
	for _, sub := range []string{"", "inputs", "outputs", "logs"} {
		p := filepath.Join(base, sub)
		if err := os.MkdirAll(p, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", p, err)
		}
	}
	return nil
}

// WriteParamsYAML serializes the block's args to params.yaml in the working directory.
func WriteParamsYAML(args map[string]any, workDir string) error {
	data, err := yaml.Marshal(args)
	if err != nil {
		return fmt.Errorf("marshaling params: %w", err)
	}
	path := filepath.Join(workDir, "params.yaml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing params.yaml: %w", err)
	}
	return nil
}

// SetupInputSymlinks creates symlinks in <workDir>/inputs/<input_name>/ pointing to
// the output files from dependency blocks.
func SetupInputSymlinks(workDir string, resolvedInputs map[string]ResolvedInput, pipelineDir string) error {
	for inputName, ri := range resolvedInputs {
		linkDir := filepath.Join(workDir, "inputs", inputName)
		if err := os.MkdirAll(linkDir, 0755); err != nil {
			return fmt.Errorf("creating input dir %s: %w", linkDir, err)
		}
		target := filepath.Join(pipelineDir, ri.SourceBlockID.String(), "outputs", ri.SourceOutputName)
		link := filepath.Join(linkDir, ri.SourceOutputName)
		if err := os.Symlink(target, link); err != nil {
			return fmt.Errorf("creating symlink %s -> %s: %w", link, target, err)
		}
	}
	return nil
}

// SetupBroadcastInputs symlinks non-mapped dependency outputs into every mapped
// invocation's inputs/ directory for map context broadcast.
func SetupBroadcastInputs(workDir string, nonMappedInputs map[string]ResolvedInput, pipelineDir string) error {
	return SetupInputSymlinks(workDir, nonMappedInputs, pipelineDir)
}

// CollectOutputs scans the outputs/ directory, computes content hashes (SHA-256) for
// each output file/directory, and returns a map of output name to hash.
func CollectOutputs(workDir string) (map[string]string, error) {
	outputsDir := filepath.Join(workDir, "outputs")
	entries, err := os.ReadDir(outputsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("reading outputs dir: %w", err)
	}

	hashes := make(map[string]string)
	for _, entry := range entries {
		p := filepath.Join(outputsDir, entry.Name())
		hash, err := ComputeContentHash(p)
		if err != nil {
			return nil, fmt.Errorf("hashing output %s: %w", entry.Name(), err)
		}
		hashes[entry.Name()] = hash
	}
	return hashes, nil
}

// ComputeContentHash computes a SHA-256 hash of a file or, for directories,
// a deterministic hash of all files within.
func ComputeContentHash(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if !info.IsDir() {
		return hashFile(path)
	}

	// For directories, hash all files in sorted order
	h := sha256.New()
	var files []string
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(path, p)
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	for _, f := range files {
		// Include the relative path in the hash for determinism
		h.Write([]byte(f))
		fh, err := hashFile(filepath.Join(path, f))
		if err != nil {
			return "", err
		}
		h.Write([]byte(fh))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DetectLanguage determines the collection language by checking for marker files.
func DetectLanguage(repoRoot string) (CollectionLanguage, error) {
	checks := []struct {
		file string
		lang CollectionLanguage
	}{
		{"Cargo.toml", CollectionLanguageRust},
		{"go.mod", CollectionLanguageGo},
		{"pyproject.toml", CollectionLanguagePython},
		{"package.json", CollectionLanguageTypeScript},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(repoRoot, c.file)); err == nil {
			return c.lang, nil
		}
	}
	return CollectionLanguageR, nil
}

// DiscoverBlocks scans the blocks/ directory for *.yaml files and returns the
// list of block manifest paths.
func DiscoverBlocks(repoRoot string) ([]string, error) {
	blocksDir := filepath.Join(repoRoot, "blocks")
	entries, err := os.ReadDir(blocksDir)
	if err != nil {
		return nil, fmt.Errorf("reading blocks directory: %w", err)
	}

	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			paths = append(paths, filepath.Join(blocksDir, entry.Name()))
		}
	}
	return paths, nil
}

// ResolveEntrypoint returns the executable path and arguments for running a block
// based on its language.
func ResolveEntrypoint(entry BlockRegistryEntry) (string, []string, error) {
	lang := CollectionLanguage(entry.Language)
	entrypoint := entry.Entrypoint
	if entrypoint == "" {
		entrypoint = entry.BlockName
	}

	switch lang {
	case CollectionLanguageRust, CollectionLanguageGo:
		// Collection binary with block name as subcommand
		binPath := filepath.Join(entry.InstalledPath, entry.CollectionName)
		return binPath, []string{entrypoint}, nil
	case CollectionLanguageTypeScript:
		// Bundled binary with block name as subcommand
		binPath := filepath.Join(entry.InstalledPath, entry.CollectionName)
		return binPath, []string{entrypoint}, nil
	case CollectionLanguagePython:
		// Block handlers live at src/<module>/<entrypoint>.py and each one
		// ends in `if __name__ == "__main__": run(handler)`, so we invoke
		// them as modules. Default the module to the collection name with
		// hyphens→underscores (setuptools convention). If `entrypoint`
		// already contains a dot, treat it as a fully-qualified module
		// path and pass it through verbatim.
		moduleSpec := entrypoint
		if !strings.Contains(moduleSpec, ".") {
			pkgModule := strings.ReplaceAll(entry.CollectionName, "-", "_")
			moduleSpec = pkgModule + "." + entrypoint
		}
		return "uv", []string{
			"run",
			"--project", entry.InstalledPath,
			"--no-sync",
			"-m", moduleSpec,
		}, nil
	case CollectionLanguageR:
		return "Rscript", []string{entrypoint}, nil
	default:
		return "", nil, fmt.Errorf("unsupported language: %s", entry.Language)
	}
}
