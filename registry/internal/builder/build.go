package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"core"
)

// The current target platform/arch (registry.md §5: Debian Linux on amd64).
const (
	TargetPlatform = "linux"
	TargetArch     = "amd64"
)

// Builder compiles/packages a cloned collection into an artifact directory. The
// returned directory is what gets packaged into <...>/<arch>.tar.gz. Builders
// run in the same base image as the worker plus toolchains (registry.md §5), so
// the artifact matches the runtime environment exactly.
type Builder interface {
	// Build produces the artifact contents in a directory and returns its path.
	Build(ctx context.Context, srcDir, collection, version string) (artifactDir string, err error)
}

// BuilderFor returns the Builder for a language, or an error for an unsupported
// language. All five supported languages (Go, Rust, TypeScript/Bun, Python, R)
// have real builders.
func BuilderFor(lang string) (Builder, error) {
	switch core.CollectionLanguage(lang) {
	case core.CollectionLanguageGo:
		return GoBuilder{}, nil
	case core.CollectionLanguageRust:
		return RustBuilder{}, nil
	case core.CollectionLanguageTypeScript:
		return BunBuilder{}, nil
	case core.CollectionLanguagePython:
		return PythonBuilder{}, nil
	case core.CollectionLanguageR:
		return RBuilder{}, nil
	default:
		return nil, fmt.Errorf("unsupported language %q", lang)
	}
}

// GoBuilder builds a Go collection: `go build` into a single binary with
// subcommands, packaged alongside the blocks/*.yaml manifests (registry.md §5,
// "Rust / Go / TypeScript: a single binary plus the blocks/*.yaml manifests").
type GoBuilder struct{}

func (GoBuilder) Build(ctx context.Context, srcDir, collection, version string) (string, error) {
	artifactDir, err := os.MkdirTemp("", "spade-artifact-*")
	if err != nil {
		return "", err
	}

	// Build the single collection binary, named after the collection.
	binName := collection
	binPath := filepath.Join(artifactDir, binName)
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./...")
	cmd.Dir = srcDir
	cmd.Env = append(os.Environ(),
		"GOOS="+TargetPlatform, "GOARCH="+TargetArch, "CGO_ENABLED=0",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(artifactDir)
		return "", fmt.Errorf("go build failed: %v\n%s", err, out)
	}

	// Copy the blocks/ manifests next to the binary.
	if err := copyBlocksDir(srcDir, artifactDir); err != nil {
		os.RemoveAll(artifactDir)
		return "", err
	}
	return artifactDir, nil
}

// copyBlocksDir copies <src>/blocks/*.yaml into <dst>/blocks/.
func copyBlocksDir(src, dst string) error {
	srcBlocks := filepath.Join(src, "blocks")
	entries, err := os.ReadDir(srcBlocks)
	if err != nil {
		return fmt.Errorf("reading blocks dir: %w", err)
	}
	dstBlocks := filepath.Join(dst, "blocks")
	if err := os.MkdirAll(dstBlocks, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(srcBlocks, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dstBlocks, e.Name()), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
