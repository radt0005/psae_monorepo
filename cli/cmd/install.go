package cmd

import (
	"core"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// copyPyprojectWithAbsoluteSources copies pyproject.toml and rewrites any
// [tool.uv.sources] `path` entries to absolute paths resolved against
// srcDir. Without this, relative paths (e.g. `../../libs/python`) break
// when pyproject.toml is relocated to ~/.spade/blocks/<collection>/<ver>/.
func copyPyprojectWithAbsoluteSources(srcDir, dstPath string) error {
	data, err := os.ReadFile(filepath.Join(srcDir, "pyproject.toml"))
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing pyproject.toml: %w", err)
	}

	if sources, ok := nestedMap(doc, "tool", "uv", "sources"); ok {
		for _, src := range sources {
			entry, ok := src.(map[string]any)
			if !ok {
				continue
			}
			path, ok := entry["path"].(string)
			if !ok || filepath.IsAbs(path) {
				continue
			}
			entry["path"] = filepath.Clean(filepath.Join(srcDir, path))
		}
	}

	out, err := toml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("serializing pyproject.toml: %w", err)
	}
	return os.WriteFile(dstPath, out, 0644)
}

// nestedMap walks a chain of string keys through nested map[string]any
// tables and returns the final table if all intermediate keys resolve.
func nestedMap(doc map[string]any, keys ...string) (map[string]any, bool) {
	cur := doc
	for _, k := range keys {
		next, ok := cur[k].(map[string]any)
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

var installCmd = &cobra.Command{
	Use:   "install <git-url | path>",
	Short: "Install a block collection from a git repository or local directory",
	Long: `Install a block collection into ~/.spade/blocks/.

The source may be a git URL or a local directory path:

  spade install https://github.com/example/gdal-blocks.git
  spade install file:///path/to/local/repo
  spade install .                         # install from current directory
  spade install ./my-collection           # install from a local path

Git URLs are shallow-cloned into a temp directory. Local paths are built
in place — no clone is performed and the directory is not a git
repository requirement.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstall(args[0])
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}

// isLocalSource classifies the install spec as either a git URL or a local
// directory path. For local paths it returns the cleaned absolute path.
// Git URLs return ("", false, nil). Invalid local paths return ("", true, err)
// so callers can distinguish "looked local but unusable" from "is a git URL".
func isLocalSource(spec string) (string, bool, error) {
	if i := strings.Index(spec, "://"); i > 0 {
		switch spec[:i] {
		case "http", "https", "git", "ssh", "file":
			return "", false, nil
		}
	}
	if strings.HasPrefix(spec, "git@") {
		return "", false, nil
	}

	abs, err := filepath.Abs(spec)
	if err != nil {
		return "", true, fmt.Errorf("resolving local path %q: %w", spec, err)
	}
	abs = filepath.Clean(abs)

	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", true, fmt.Errorf("local path does not exist: %s", abs)
		}
		return "", true, fmt.Errorf("stat %q: %w", abs, err)
	}
	if !info.IsDir() {
		return "", true, fmt.Errorf("local path is not a directory: %s", abs)
	}
	return abs, true, nil
}

func runInstall(spec string) error {
	absPath, local, err := isLocalSource(spec)
	if err != nil && local {
		return err
	}

	if local {
		fmt.Printf("Installing from local directory: %s\n", absPath)
		return installFromSource(absPath, false)
	}

	tmpDir, err := os.MkdirTemp("", "spade-install-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}

	fmt.Printf("Cloning %s...\n", spec)
	cloneCmd := exec.Command("git", "clone", "--depth=1", spec, tmpDir)
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("cloning repository: %w", err)
	}

	return installFromSource(tmpDir, true)
}

func installFromSource(srcDir string, cleanupSrc bool) error {
	if cleanupSrc {
		defer os.RemoveAll(srcDir)
	}

	lang, err := core.DetectLanguage(srcDir)
	if err != nil {
		return fmt.Errorf("detecting language: %w", err)
	}
	fmt.Printf("Detected language: %s\n", lang)

	blockPaths, err := core.DiscoverBlocks(srcDir)
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

	collectionName, err := ReadCollectionName(srcDir, lang)
	if err != nil {
		collectionName = filepath.Base(srcDir)
	}
	collectionVersion, err := ReadCollectionVersion(srcDir, lang)
	if err != nil {
		collectionVersion = "0.1.0"
	}
	fmt.Printf("Collection: %s v%s\n", collectionName, collectionVersion)

	fmt.Println("Building...")
	if err := runBuild(srcDir, lang, collectionName); err != nil {
		return fmt.Errorf("building collection: %w", err)
	}

	installDir := filepath.Join(BlocksDir(), collectionName, collectionVersion)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("creating install directory: %w", err)
	}

	if err := copyCollection(srcDir, installDir, lang, collectionName); err != nil {
		return fmt.Errorf("copying collection: %w", err)
	}

	if lang == core.CollectionLanguagePython {
		if err := finalizePythonInstall(installDir); err != nil {
			return fmt.Errorf("finalizing python install: %w", err)
		}
	}

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

	return nil
}

// finalizePythonInstall creates the venv inside the install directory so
// workers can invoke blocks via `uv run --project <installDir>` without
// touching the original source. We run a fresh `uv sync` (not --frozen)
// because the copied uv.lock still stores editable [tool.uv.sources]
// paths relative to the original project root; uv re-resolves those
// against the rewritten absolute paths in pyproject.toml. All other
// pins come from the cache, so this is fast.
func finalizePythonInstall(installDir string) error {
	fmt.Println("Creating environment in install directory...")
	cmd := exec.Command("uv", "sync")
	cmd.Dir = installDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("uv sync failed: %w", err)
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
		// Copy pyproject.toml, uv.lock (if present), and src/ so the install
		// dir is a complete, uv-syncable project. The venv is built there by
		// finalizePythonInstall — not copied — because uv venvs carry
		// absolute paths that would not survive a relocation.
		if err := copyPyprojectWithAbsoluteSources(
			srcDir,
			filepath.Join(dstDir, "pyproject.toml"),
		); err != nil {
			return fmt.Errorf("copying pyproject.toml: %w", err)
		}
		lockSrc := filepath.Join(srcDir, "uv.lock")
		if _, err := os.Stat(lockSrc); err == nil {
			if err := copyFileSimple(lockSrc, filepath.Join(dstDir, "uv.lock")); err != nil {
				return fmt.Errorf("copying uv.lock: %w", err)
			}
		}
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
