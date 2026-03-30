package cmd

import (
	"archive/tar"
	"compress/gzip"
	"core"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload a block collection for security screening and cloud use",
	Long:  `Validates and packages the current collection for upload to the cloud system.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpload()
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)
}

func runUpload() error {
	// Step 1: Validate
	errs := ValidateCollection(".")
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Collection validation failed with %d error(s):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		os.Exit(1)
	}

	// Step 2: Detect language and read metadata
	lang, err := core.DetectLanguage(".")
	if err != nil {
		return fmt.Errorf("detecting language: %w", err)
	}

	name, err := ReadCollectionName(".", lang)
	if err != nil {
		name = filepath.Base(mustGetwd())
	}
	version, err := ReadCollectionVersion(".", lang)
	if err != nil {
		version = "0.1.0"
	}

	// Step 3: Package
	archiveName := fmt.Sprintf("%s-%s.tar.gz", name, version)
	if err := createArchive(archiveName, ".", lang); err != nil {
		return fmt.Errorf("creating archive: %w", err)
	}

	fmt.Printf("Collection packaged: %s\n", archiveName)
	fmt.Println("Note: Upload endpoint is not yet configured. The server-side upload API will be integrated when the PocketBase server is available.")
	return nil
}

func createArchive(archiveName string, dir string, lang core.CollectionLanguage) error {
	outFile, err := os.Create(archiveName)
	if err != nil {
		return err
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Always include blocks/ directory
	blocksDir := filepath.Join(dir, "blocks")
	if err := addDirToTar(tarWriter, blocksDir, dir); err != nil {
		return fmt.Errorf("adding blocks: %w", err)
	}

	// Include language manifest
	langManifests := map[core.CollectionLanguage]string{
		core.CollectionLanguageRust:       "Cargo.toml",
		core.CollectionLanguagePython:     "pyproject.toml",
		core.CollectionLanguageGo:         "go.mod",
		core.CollectionLanguageTypeScript: "package.json",
		core.CollectionLanguageR:          "renv.lock",
	}
	if manifestFile, ok := langManifests[lang]; ok {
		addFileToTar(tarWriter, filepath.Join(dir, manifestFile), dir)
	}

	// Include source files
	srcDirs := map[core.CollectionLanguage]string{
		core.CollectionLanguageRust:       "src",
		core.CollectionLanguagePython:     "src",
		core.CollectionLanguageGo:         ".",
		core.CollectionLanguageTypeScript: "src",
		core.CollectionLanguageR:          "R",
	}
	if srcDir, ok := srcDirs[lang]; ok {
		if srcDir == "." {
			// For Go, add all .go files in root
			entries, _ := os.ReadDir(dir)
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
					addFileToTar(tarWriter, filepath.Join(dir, e.Name()), dir)
				}
			}
		} else {
			addDirToTar(tarWriter, filepath.Join(dir, srcDir), dir)
		}
	}

	return nil
}

func addDirToTar(tw *tar.Writer, dir string, baseDir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return addFileToTar(tw, path, baseDir)
	})
}

func addFileToTar(tw *tar.Writer, path string, baseDir string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}

	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    rel,
		Size:    info.Size(),
		Mode:    int64(info.Mode()),
		ModTime: info.ModTime(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(tw, file)
	return err
}
