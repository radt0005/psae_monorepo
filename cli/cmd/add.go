package cmd

import (
	"core"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new block to the current collection",
	Long:  `Creates a block manifest and entrypoint file for a new block in the current collection.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAdd(args[0])
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(name string) error {
	lang, err := core.DetectLanguage(".")
	if err != nil {
		return fmt.Errorf("detecting language: %w (are you in a collection directory?)", err)
	}

	dirName := filepath.Base(mustGetwd())
	blockID := dirName + "." + name

	// Create block manifest
	manifest := core.BlockManifest{
		ID:          blockID,
		Version:     "0.1.0",
		Kind:        core.BlockKindStandard,
		Network:     false,
		Description: "",
		Entrypoint:  name,
		Inputs:      map[string]core.InputDeclaration{},
		Outputs:     map[string]core.OutputDeclaration{},
	}

	if err := os.MkdirAll("blocks", 0755); err != nil {
		return fmt.Errorf("creating blocks directory: %w", err)
	}

	manifestPath := filepath.Join("blocks", name+".yaml")
	data, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}
	fmt.Printf("  Created %s\n", manifestPath)

	// Create entrypoint file
	if err := createEntrypoint(name, lang); err != nil {
		return err
	}

	return nil
}

func createEntrypoint(name string, lang core.CollectionLanguage) error {
	switch lang {
	case core.CollectionLanguageRust:
		path := filepath.Join("src", name+".rs")
		if err := os.MkdirAll("src", 0755); err != nil {
			return err
		}
		content := fmt.Sprintf(`/// Block: %s
pub fn run() {
    // TODO: implement block logic
}
`, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
		fmt.Printf("  Created %s\n", path)
		fmt.Println("  Note: register this module in src/lib.rs or src/main.rs")

	case core.CollectionLanguageGo:
		path := name + ".go"
		content := fmt.Sprintf(`package main

// %s is the entry point for the %s block.
func %s() {
	// TODO: implement block logic
}
`, name, name, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
		fmt.Printf("  Created %s\n", path)

	case core.CollectionLanguagePython:
		// Find the package directory under src/
		pkgDir, err := findPythonPackageDir()
		if err != nil {
			return err
		}
		path := filepath.Join(pkgDir, name+".py")
		content := fmt.Sprintf(`"""Block: %s"""
import yaml


def handler(params):
    """Process inputs and write outputs."""
    # TODO: implement block logic
    pass


if __name__ == "__main__":
    with open("params.yaml") as f:
        params = yaml.safe_load(f)
    handler(params)
`, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
		fmt.Printf("  Created %s\n", path)

	case core.CollectionLanguageTypeScript:
		path := filepath.Join("src", name+".ts")
		if err := os.MkdirAll("src", 0755); err != nil {
			return err
		}
		content := fmt.Sprintf(`// Block: %s

export function handler(params: Record<string, unknown>): void {
  // TODO: implement block logic
}
`, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
		fmt.Printf("  Created %s\n", path)

	case core.CollectionLanguageR:
		path := filepath.Join("R", name+".R")
		if err := os.MkdirAll("R", 0755); err != nil {
			return err
		}
		content := fmt.Sprintf(`# Block: %s

library(yaml)

params <- read_yaml("params.yaml")

# TODO: implement block logic

# Write outputs to outputs/ directory
`, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
		fmt.Printf("  Created %s\n", path)
	}

	return nil
}

func findPythonPackageDir() (string, error) {
	srcDir := "src"
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return "", fmt.Errorf("reading src/ directory: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && !strings.HasPrefix(e.Name(), "_") {
			return filepath.Join(srcDir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no Python package directory found under src/")
}
