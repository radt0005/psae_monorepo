package cmd

import (
	"core"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadCollectionName reads the collection name from the language manifest.
func ReadCollectionName(repoRoot string, lang core.CollectionLanguage) (string, error) {
	switch lang {
	case core.CollectionLanguageRust:
		return readCargoName(repoRoot)
	case core.CollectionLanguagePython:
		return readPyprojectName(repoRoot)
	case core.CollectionLanguageTypeScript:
		return readPackageJSONName(repoRoot)
	case core.CollectionLanguageGo:
		return readGoModName(repoRoot)
	case core.CollectionLanguageR:
		return filepath.Base(repoRoot), nil
	default:
		return filepath.Base(repoRoot), nil
	}
}

// ReadCollectionVersion reads the version from the language manifest.
func ReadCollectionVersion(repoRoot string, lang core.CollectionLanguage) (string, error) {
	switch lang {
	case core.CollectionLanguageRust:
		return readCargoVersion(repoRoot)
	case core.CollectionLanguagePython:
		return readPyprojectVersion(repoRoot)
	case core.CollectionLanguageTypeScript:
		return readPackageJSONVersion(repoRoot)
	case core.CollectionLanguageGo:
		return "0.1.0", nil // Go modules don't have a version in go.mod by convention
	case core.CollectionLanguageR:
		return "0.1.0", nil
	default:
		return "0.1.0", nil
	}
}

func readCargoName(repoRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "Cargo.toml"))
	if err != nil {
		return "", fmt.Errorf("reading Cargo.toml: %w", err)
	}
	return parseTOMLValue(string(data), "name")
}

func readCargoVersion(repoRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "Cargo.toml"))
	if err != nil {
		return "", fmt.Errorf("reading Cargo.toml: %w", err)
	}
	return parseTOMLValue(string(data), "version")
}

func readPyprojectName(repoRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "pyproject.toml"))
	if err != nil {
		return "", fmt.Errorf("reading pyproject.toml: %w", err)
	}
	return parseTOMLValue(string(data), "name")
}

func readPyprojectVersion(repoRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "pyproject.toml"))
	if err != nil {
		return "", fmt.Errorf("reading pyproject.toml: %w", err)
	}
	return parseTOMLValue(string(data), "version")
}

func readPackageJSONName(repoRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "package.json"))
	if err != nil {
		return "", fmt.Errorf("reading package.json: %w", err)
	}
	var pkg map[string]any
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("parsing package.json: %w", err)
	}
	name, ok := pkg["name"].(string)
	if !ok {
		return "", fmt.Errorf("package.json missing name field")
	}
	return name, nil
}

func readPackageJSONVersion(repoRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "package.json"))
	if err != nil {
		return "", fmt.Errorf("reading package.json: %w", err)
	}
	var pkg map[string]any
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("parsing package.json: %w", err)
	}
	version, ok := pkg["version"].(string)
	if !ok {
		return "", fmt.Errorf("package.json missing version field")
	}
	return version, nil
}

func readGoModName(repoRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("reading go.mod: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			mod := strings.TrimPrefix(line, "module ")
			mod = strings.TrimSpace(mod)
			// Use the last path segment as the name
			parts := strings.Split(mod, "/")
			return parts[len(parts)-1], nil
		}
	}
	return filepath.Base(repoRoot), nil
}

// parseTOMLValue does a simple line-based extraction of a key = "value" pair from TOML.
// This avoids pulling in a full TOML parser dependency for simple metadata extraction.
func parseTOMLValue(content string, key string) (string, error) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key) {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "\"")
			return val, nil
		}
	}
	return "", fmt.Errorf("key %q not found", key)
}
