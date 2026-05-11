package cmd

import (
	"core"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check [pipeline.yaml]",
	Short: "Validate a pipeline or block collection",
	Long: `Validates a pipeline file or block collection.

With an argument: validates a pipeline YAML file.
Without arguments: validates all block manifests in the current collection directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return runCheckPipeline(args[0])
		}
		return runCheckCollection()
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

func runCheckPipeline(pipelinePath string) error {
	pipeline, _, wroteLockfile, err := core.LoadAndResolvePipeline(pipelinePath)
	if err != nil {
		if errors.Is(err, core.ErrInvalidLockfile) {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			fmt.Fprintf(os.Stderr, "To regenerate the lockfile from scratch, delete %s.\n",
				core.LockfilePathFor(pipelinePath))
			os.Exit(1)
		}
		return fmt.Errorf("loading pipeline: %w", err)
	}
	if wroteLockfile {
		fmt.Printf("Wrote %s\n", core.LockfilePathFor(pipelinePath))
	}

	registry, err := core.OpenRegistry(RegistryPath())
	if err != nil {
		return fmt.Errorf("opening registry: %w", err)
	}
	defer registry.Close()

	// Build manifest map from registry
	manifests := make(map[string]core.BlockManifest)
	for _, block := range pipeline.Blocks {
		if _, exists := manifests[block.Name]; exists {
			continue
		}
		entry, err := registry.LookupBlock(block.Name, "")
		if err != nil {
			return fmt.Errorf("block type %q not found in registry: %w", block.Name, err)
		}
		manifestPath := findManifestForBlock(entry)
		manifest, err := core.LoadBlockManifest(manifestPath)
		if err != nil {
			return fmt.Errorf("loading manifest for %q: %w", block.Name, err)
		}
		manifests[block.Name] = manifest
	}

	// Run validation
	errs := core.ValidatePipeline(pipeline, manifests)
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Pipeline validation failed with %d error(s):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		os.Exit(1)
	}

	fmt.Printf("Pipeline %q is valid.\n", pipeline.Name)
	return nil
}

func runCheckCollection() error {
	errs := ValidateCollection(".")
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Collection validation failed with %d error(s):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		os.Exit(1)
	}

	blockPaths, _ := core.DiscoverBlocks(".")
	fmt.Printf("Collection is valid. %d block(s) checked.\n", len(blockPaths))
	return nil
}

// findManifestForBlock returns the path to the block's YAML manifest in the installed directory.
func findManifestForBlock(entry *core.BlockRegistryEntry) string {
	// The block ID follows <collection>.<block> convention
	parts := strings.SplitN(entry.BlockID, ".", 2)
	blockName := entry.BlockName
	if len(parts) == 2 {
		blockName = parts[1]
	}
	return fmt.Sprintf("%s/blocks/%s.yaml", entry.InstalledPath, blockName)
}
