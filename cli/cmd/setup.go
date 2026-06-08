package cmd

import (
	"core"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rebuildIndex bool

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up the Spade system on the local machine",
	Long:  `Creates the ~/.spade/ directory structure and initializes the block registry.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSetup()
	},
}

func init() {
	setupCmd.Flags().BoolVar(&rebuildIndex, "rebuild-index", false, "Rebuild the block registry from the filesystem")
	rootCmd.AddCommand(setupCmd)
}

func runSetup() error {
	dirs := []string{
		SpadeDir(),
		BlocksDir(),
		CacheDir(),
		PipelinesDir(),
		AuthDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
		fmt.Printf("  Created %s\n", dir)
	}

	registry, err := core.OpenRegistry(RegistryPath())
	if err != nil {
		return fmt.Errorf("initializing registry: %w", err)
	}
	defer registry.Close()
	fmt.Printf("  Initialized registry at %s\n", RegistryPath())

	if rebuildIndex {
		if err := registry.RebuildFromFilesystem(BlocksDir()); err != nil {
			return fmt.Errorf("rebuilding registry: %w", err)
		}
		fmt.Println("  Rebuilt block registry from filesystem")
	}

	fmt.Println("Spade setup complete.")
	return nil
}
