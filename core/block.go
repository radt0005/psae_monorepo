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
func CreateBlockDirectory(invocationID string, workdir string) error {
	base := filepath.Join(workdir, invocationID)
	// Mode 0777 so the isolate sandbox user (typically remapped to
	// uid 100000 via user-namespace subuid) can write to inputs/outputs/logs
	// even though the work dir itself is owned by the invoking user.
	for _, sub := range []string{"", "inputs", "outputs", "logs"} {
		p := filepath.Join(base, sub)
		if err := os.MkdirAll(p, 0777); err != nil {
			return fmt.Errorf("creating directory %s: %w", p, err)
		}
		if err := os.Chmod(p, 0777); err != nil {
			return fmt.Errorf("chmod %s: %w", p, err)
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
// the output files from dependency blocks.  The outputs of a block live under
// <workDir>/outputs/<output_name>/, so each individual file inside that
// directory is symlinked into the current block's inputs/<input_name>/.
// Using file-level symlinks (rather than linking the whole output directory)
// lets the runtime scanner find the files via plain `is_file()` checks
// without following symlinks itself.
//
// Map/reduce semantics:
//   - When the source block is a map block and the current invocation has a
//     MapIndex, the expansion manifest is read and items[MapIndex] is
//     symlinked — not the expansion.yaml itself.
//   - When the current block is a reduce block, outputs from every mapped
//     sibling invocation (<sourceID>.0, .1, …) are gathered into the input
//     directory.
//   - When the current invocation has a MapIndex and the source block was
//     also run in the same map context, the peer's work dir (<sourceID>.<idx>)
//     is used; broadcast dependencies fall back to <sourceID>.
func SetupInputSymlinks(
	workDir string,
	resolvedInputs map[string]ResolvedInput,
	pipelineDir string,
	currentInvocation BlockInvocation,
	currentManifest BlockManifest,
	depManifests map[uuid.UUID]BlockManifest,
) error {
	for inputName, ri := range resolvedInputs {
		linkDir := filepath.Join(workDir, "inputs", inputName)
		if err := os.MkdirAll(linkDir, 0777); err != nil {
			return fmt.Errorf("creating input dir %s: %w", linkDir, err)
		}
		_ = os.Chmod(linkDir, 0777)

		depManifest := depManifests[ri.SourceBlockID]
		srcUUID := ri.SourceBlockID.String()

		// Case 1: source is a map block → this mapped invocation consumes
		// one item from the expansion.
		if depManifest.Kind == BlockKindMap && currentInvocation.MapIndex != nil {
			expPath := filepath.Join(pipelineDir, srcUUID, "outputs", ri.SourceOutputName, "expansion.yaml")
			exp, err := LoadExpansionManifest(expPath)
			if err != nil {
				return fmt.Errorf("reading expansion for mapped input %q: %w", inputName, err)
			}
			idx := *currentInvocation.MapIndex
			if idx < 0 || idx >= len(exp.Items) {
				return fmt.Errorf("map index %d out of range for %s (%d items)", idx, inputName, len(exp.Items))
			}
			item := exp.Items[idx]
			target := item.Path
			if !filepath.IsAbs(target) {
				// Expansion paths are written relative to the map block's
				// work directory.
				target = filepath.Join(pipelineDir, srcUUID, target)
			}
			base := filepath.Base(target)
			link := filepath.Join(linkDir, base)
			if err := os.Symlink(target, link); err != nil {
				return fmt.Errorf("creating symlink %s -> %s: %w", link, target, err)
			}
			continue
		}

		// Case 2: current block is a reduce block → gather every mapped
		// sibling's output under inputs/<inputName>/.  Each file is prefixed
		// by the sibling's map index to avoid name collisions.
		if currentManifest.Kind == BlockKindReduce {
			siblingDirs, _ := filepath.Glob(filepath.Join(pipelineDir, srcUUID+".*"))
			sort.Strings(siblingDirs)
			if len(siblingDirs) > 0 {
				for _, sibling := range siblingDirs {
					outDir := filepath.Join(sibling, "outputs", ri.SourceOutputName)
					entries, err := os.ReadDir(outDir)
					if err != nil {
						continue
					}
					// Extract the ".N" suffix to prefix filenames with.
					suffix := filepath.Base(sibling)
					idxTag := suffix
					if dot := strings.LastIndex(suffix, "."); dot >= 0 {
						idxTag = suffix[dot+1:]
					}
					for _, entry := range entries {
						target := filepath.Join(outDir, entry.Name())
						// Prefix with "<idx>_" so collisions across siblings
						// become distinct filenames.
						link := filepath.Join(linkDir, idxTag+"_"+entry.Name())
						if err := os.Symlink(target, link); err != nil {
							return fmt.Errorf("creating symlink %s -> %s: %w", link, target, err)
						}
					}
				}
				continue
			}
			// Fall through to the default case if no mapped siblings exist.
		}

		// Case 3 / 4: default lookup.  If the current invocation is mapped
		// and a peer work dir exists at <srcUUID>.<idx>, use it; otherwise
		// use <srcUUID> (non-mapped or broadcast).
		sourceDir := filepath.Join(pipelineDir, srcUUID, "outputs", ri.SourceOutputName)
		if currentInvocation.MapIndex != nil {
			peer := filepath.Join(pipelineDir, fmt.Sprintf("%s.%d", srcUUID, *currentInvocation.MapIndex))
			if info, err := os.Stat(peer); err == nil && info.IsDir() {
				sourceDir = filepath.Join(peer, "outputs", ri.SourceOutputName)
			}
		}
		entries, err := os.ReadDir(sourceDir)
		if err != nil {
			return fmt.Errorf("reading source output dir %s: %w", sourceDir, err)
		}
		for _, entry := range entries {
			target := filepath.Join(sourceDir, entry.Name())
			link := filepath.Join(linkDir, entry.Name())
			if err := os.Symlink(target, link); err != nil {
				return fmt.Errorf("creating symlink %s -> %s: %w", link, target, err)
			}
		}
	}
	return nil
}

// SetupBroadcastInputs symlinks non-mapped dependency outputs into every mapped
// invocation's inputs/ directory for map context broadcast.  It delegates to
// SetupInputSymlinks with empty block/manifest context (treated as case 4).
func SetupBroadcastInputs(workDir string, nonMappedInputs map[string]ResolvedInput, pipelineDir string) error {
	return SetupInputSymlinks(
		workDir,
		nonMappedInputs,
		pipelineDir,
		BlockInvocation{},
		BlockManifest{},
		map[uuid.UUID]BlockManifest{},
	)
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
		// Skip symlinks: they reference content rather than being content
		// themselves, and a symlink-to-dir would fail to hash as a file.
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
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
		// R block scripts are installed at <installedPath>/R/<entrypoint>.R.
		// The entrypoint defaults to the block name (the filename stem), so we
		// must expand it to the absolute script path — Rscript receives a bare
		// name otherwise and fails to find the file in the work directory.
		scriptPath := filepath.Join(entry.InstalledPath, "R", entrypoint+".R")
		return "Rscript", []string{scriptPath}, nil
	default:
		return "", nil, fmt.Errorf("unsupported language: %s", entry.Language)
	}
}
