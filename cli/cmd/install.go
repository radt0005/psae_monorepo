package cmd

import (
	"core"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install <git-url>",
	Short: "Install a block collection from a git repository",
	Long:  `Clones a git repository, builds the collection, and installs it to ~/.spade/blocks/.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstall(args[0])
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func runInstall(gitURL string) error {
	// Step 1: Clone repository
	tmpDir, err := os.MkdirTemp("", "spade-install-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Printf("Cloning %s...\n", gitURL)
	cloneCmd := exec.Command("git", "clone", "--depth=1", gitURL, tmpDir)
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("cloning repository: %w", err)
	}

	// Step 2: Detect language
	lang, err := core.DetectLanguage(tmpDir)
	if err != nil {
		return fmt.Errorf("detecting language: %w", err)
	}
	fmt.Printf("Detected language: %s\n", lang)

	// Step 3: Discover blocks
	blockPaths, err := core.DiscoverBlocks(tmpDir)
	if err != nil {
		return fmt.Errorf("discovering blocks: %w", err)
	}
	if len(blockPaths) == 0 {
		return fmt.Errorf("no block manifests found in blocks/ directory")
	}

	var manifests []core.BlockManifest
	for _, bp := range blockPaths {
		m, err := core.LoadBlockManifest(bp)
		if err != nil {
			return fmt.Errorf("loading manifest %s: %w", bp, err)
		}
		manifests = append(manifests, m)
	}

	// Step 4: Extract version and collection name
	collectionName, err := ReadCollectionName(tmpDir, lang)
	if err != nil {
		collectionName = filepath.Base(tmpDir)
	}
	collectionVersion, err := ReadCollectionVersion(tmpDir, lang)
	if err != nil {
		collectionVersion = "0.1.0"
	}
	fmt.Printf("Collection: %s v%s\n", collectionName, collectionVersion)

	// Step 5: Run language-specific build
	fmt.Println("Building...")
	if err := runBuild(tmpDir, lang, collectionName); err != nil {
		return fmt.Errorf("building collection: %w", err)
	}

	// Step 6: Install to ~/.spade/blocks/
	installDir := filepath.Join(BlocksDir(), collectionName, collectionVersion)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("creating install directory: %w", err)
	}

	if err := copyCollection(tmpDir, installDir, lang, collectionName); err != nil {
		return fmt.Errorf("copying collection: %w", err)
	}

	// Step 7: Register in block registry
	registry, err := core.OpenRegistry(RegistryPath())
	if err != nil {
		return fmt.Errorf("opening registry: %w", err)
	}
	defer registry.Close()

	contentHash, err := core.ComputeContentHash(installDir)
	if err != nil {
		contentHash = ""
	}

	for _, manifest := range manifests {
		entry := core.BlockRegistryEntry{
			CollectionName:    collectionName,
			CollectionVersion: collectionVersion,
			BlockName:         manifest.ID,
			BlockID:           manifest.ID,
			Language:          string(lang),
			Entrypoint:        manifest.Entrypoint,
			InstalledPath:     installDir,
			ContentHash:       contentHash,
			Kind:              string(manifest.Kind),
			Network:           manifest.Network,
		}
		if err := registry.RegisterBlock(entry); err != nil {
			return fmt.Errorf("registering block %s: %w", manifest.ID, err)
		}
	}

	fmt.Printf("Installed %d block(s) to %s\n", len(manifests), installDir)
	return nil
}

func runBuild(dir string, lang core.CollectionLanguage, name string) error {
	var cmd *exec.Cmd

	switch lang {
	case core.CollectionLanguageRust:
		cmd = exec.Command("cargo", "build", "--release")
	case core.CollectionLanguageGo:
		cmd = exec.Command("go", "build", "-o", name)
	case core.CollectionLanguagePython:
		cmd = exec.Command("uv", "sync")
	case core.CollectionLanguageTypeScript:
		cmd = exec.Command("bun", "build", ".")
	case core.CollectionLanguageR:
		if _, err := os.Stat(filepath.Join(dir, "setup.R")); err == nil {
			cmd = exec.Command("Rscript", "setup.R")
		} else {
			// No build step needed for R
			return nil
		}
	default:
		return fmt.Errorf("unsupported language: %s", lang)
	}

	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// For Python, also install as a tool
	if lang == core.CollectionLanguagePython {
		installCmd := exec.Command("uv", "tool", "install", ".")
		installCmd.Dir = dir
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("uv tool install failed: %w", err)
		}
	}

	return nil
}

func copyCollection(srcDir, dstDir string, lang core.CollectionLanguage, name string) error {
	// Copy block manifests
	srcBlocks := filepath.Join(srcDir, "blocks")
	dstBlocks := filepath.Join(dstDir, "blocks")
	if err := os.MkdirAll(dstBlocks, 0755); err != nil {
		return err
	}

	blockFiles, err := os.ReadDir(srcBlocks)
	if err != nil {
		return fmt.Errorf("reading blocks directory: %w", err)
	}
	for _, f := range blockFiles {
		if f.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(srcBlocks, f.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dstBlocks, f.Name()), data, 0644); err != nil {
			return err
		}
	}

	// Copy build artifacts based on language
	switch lang {
	case core.CollectionLanguageRust:
		binSrc := filepath.Join(srcDir, "target", "release", name)
		binDst := filepath.Join(dstDir, name)
		return copyFileSimple(binSrc, binDst)
	case core.CollectionLanguageGo:
		binSrc := filepath.Join(srcDir, name)
		binDst := filepath.Join(dstDir, name)
		return copyFileSimple(binSrc, binDst)
	case core.CollectionLanguagePython:
		// Copy the whole src and pyproject.toml
		return copyDirRecursive(filepath.Join(srcDir, "src"), filepath.Join(dstDir, "src"))
	case core.CollectionLanguageTypeScript:
		binSrc := filepath.Join(srcDir, name)
		binDst := filepath.Join(dstDir, name)
		return copyFileSimple(binSrc, binDst)
	case core.CollectionLanguageR:
		// Copy R/ directory
		return copyDirRecursive(filepath.Join(srcDir, "R"), filepath.Join(dstDir, "R"))
	}

	return nil
}

func copyFileSimple(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

func copyDirRecursive(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
