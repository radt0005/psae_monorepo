package cmd

import (
	"core"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validInputTypes are the allowed input types in block manifests.
var validInputTypes = map[string]bool{
	"file":       true,
	"directory":  true,
	"collection": true,
	"string":     true,
	"number":     true,
	"boolean":    true,
}

// validOutputTypes are the allowed output types in block manifests.
var validOutputTypes = map[string]bool{
	"file":       true,
	"directory":  true,
	"collection": true,
	"json":       true,
	"expansion":  true,
}

// ValidateCollection runs all collection-level validation checks on a directory.
func ValidateCollection(dir string) []error {
	var errs []error

	lang, err := core.DetectLanguage(dir)
	if err != nil {
		return []error{fmt.Errorf("detecting language: %w", err)}
	}

	blockPaths, err := core.DiscoverBlocks(dir)
	if err != nil {
		return []error{fmt.Errorf("discovering blocks: %w", err)}
	}

	if len(blockPaths) == 0 {
		return []error{fmt.Errorf("no block manifests found in blocks/ directory")}
	}

	for _, blockPath := range blockPaths {
		manifest, err := core.LoadBlockManifest(blockPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("loading %s: %w", blockPath, err))
			continue
		}

		blockName := strings.TrimSuffix(filepath.Base(blockPath), ".yaml")
		errs = append(errs, validateBlockManifest(manifest, blockName, dir, lang)...)
	}

	return errs
}

func validateBlockManifest(manifest core.BlockManifest, blockName string, dir string, lang core.CollectionLanguage) []error {
	var errs []error

	// 1. Required fields
	if manifest.ID == "" {
		errs = append(errs, fmt.Errorf("block %s: missing required field 'id'", blockName))
	}
	if manifest.Version == "" {
		errs = append(errs, fmt.Errorf("block %s: missing required field 'version'", blockName))
	}
	if manifest.Inputs == nil {
		errs = append(errs, fmt.Errorf("block %s: missing required field 'inputs'", blockName))
	}
	if manifest.Outputs == nil {
		errs = append(errs, fmt.Errorf("block %s: missing required field 'outputs'", blockName))
	}

	// 2. Valid input types
	for name, input := range manifest.Inputs {
		if !validInputTypes[input.Type] {
			errs = append(errs, fmt.Errorf("block %s: input %q has invalid type %q", blockName, name, input.Type))
		}
	}

	// 2. Valid output types
	for name, output := range manifest.Outputs {
		if !validOutputTypes[output.Type] {
			errs = append(errs, fmt.Errorf("block %s: output %q has invalid type %q", blockName, name, output.Type))
		}
	}

	// 3. Block ID convention
	if manifest.ID != "" && !strings.Contains(manifest.ID, ".") {
		errs = append(errs, fmt.Errorf("block %s: id %q should follow <collection>.<block> convention", blockName, manifest.ID))
	}

	// 4. Entrypoint resolves to existing file
	errs = append(errs, validateEntrypoint(manifest, blockName, dir, lang)...)

	// 5. Map blocks must have expansion output
	if manifest.Kind == core.BlockKindMap {
		hasExpansion := false
		for _, out := range manifest.Outputs {
			if out.Type == "expansion" {
				hasExpansion = true
				break
			}
		}
		if !hasExpansion {
			errs = append(errs, fmt.Errorf("block %s: map block must have an expansion output", blockName))
		}
	}

	// 6. Reduce blocks must have collection input
	if manifest.Kind == core.BlockKindReduce {
		hasCollection := false
		for _, in := range manifest.Inputs {
			if in.Type == "collection" {
				hasCollection = true
				break
			}
		}
		if !hasCollection {
			errs = append(errs, fmt.Errorf("block %s: reduce block must have a collection input", blockName))
		}
	}

	return errs
}

func validateEntrypoint(manifest core.BlockManifest, blockName string, dir string, lang core.CollectionLanguage) []error {
	entrypoint := manifest.Entrypoint
	if entrypoint == "" {
		entrypoint = blockName
	}

	var checkPath string
	switch lang {
	case core.CollectionLanguageRust:
		checkPath = filepath.Join(dir, "src", entrypoint+".rs")
	case core.CollectionLanguageGo:
		checkPath = filepath.Join(dir, entrypoint+".go")
	case core.CollectionLanguagePython:
		// Check in src/<package>/<name>.py - we try to find the package dir
		srcDir := filepath.Join(dir, "src")
		entries, err := os.ReadDir(srcDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
					candidate := filepath.Join(srcDir, e.Name(), entrypoint+".py")
					if _, err := os.Stat(candidate); err == nil {
						return nil // found it
					}
				}
			}
		}
		checkPath = filepath.Join(dir, "src", entrypoint+".py")
	case core.CollectionLanguageTypeScript:
		checkPath = filepath.Join(dir, "src", entrypoint+".ts")
	case core.CollectionLanguageR:
		checkPath = filepath.Join(dir, "R", entrypoint+".R")
	default:
		return nil
	}

	if _, err := os.Stat(checkPath); os.IsNotExist(err) {
		return []error{fmt.Errorf("block %s: entrypoint file %s does not exist", blockName, checkPath)}
	}
	return nil
}
