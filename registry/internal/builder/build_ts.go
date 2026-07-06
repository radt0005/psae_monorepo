package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// BunBuilder builds a TypeScript (Bun) collection into a single self-contained
// executable via `bun build --compile`, packaged alongside the blocks/*.yaml
// manifests (registry.md §5). The compiled binary embeds the Bun runtime, so the
// worker needs no `bun`/`node` at execution time — it invokes it exactly like
// the compiled languages: `<dir>/<collection> <block>` (core.ResolveEntrypoint).
type BunBuilder struct{}

func (BunBuilder) Build(ctx context.Context, srcDir, collection, version string) (string, error) {
	artifactDir, err := os.MkdirTemp("", "spade-artifact-*")
	if err != nil {
		return "", err
	}

	entry, err := bunEntrypoint(srcDir)
	if err != nil {
		os.RemoveAll(artifactDir)
		return "", err
	}

	// Resolve dependencies, then compile the collection to a standalone binary.
	if out, err := runIn(ctx, srcDir, "bun", "install", "--frozen-lockfile"); err != nil {
		// Fall back to a non-frozen install when no lockfile is committed.
		if out2, err2 := runIn(ctx, srcDir, "bun", "install"); err2 != nil {
			os.RemoveAll(artifactDir)
			return "", fmt.Errorf("bun install failed: %v\n%s%s", err2, out, out2)
		}
	}

	binPath := filepath.Join(artifactDir, collection)
	if out, err := runIn(ctx, srcDir, "bun", "build", entry, "--compile", "--outfile", binPath); err != nil {
		os.RemoveAll(artifactDir)
		return "", fmt.Errorf("bun build failed: %v\n%s", err, out)
	}

	if err := copyBlocksDir(srcDir, artifactDir); err != nil {
		os.RemoveAll(artifactDir)
		return "", err
	}
	return artifactDir, nil
}

// bunEntrypoint resolves the collection's entry module from package.json
// (`module` then `main`), falling back to conventional locations. The entry is
// the dispatcher that reads argv for the block subcommand.
func bunEntrypoint(srcDir string) (string, error) {
	var pkg struct {
		Module string `json:"module"`
		Main   string `json:"main"`
	}
	if data, err := os.ReadFile(filepath.Join(srcDir, "package.json")); err == nil {
		_ = json.Unmarshal(data, &pkg)
	}
	candidates := []string{pkg.Module, pkg.Main, "src/index.ts", "index.ts", "src/main.ts", "index.js"}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(srcDir, c)); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("no entrypoint found (looked for package.json module/main, src/index.ts, index.ts)")
}

// runIn runs a command in dir and returns combined output for diagnostics.
func runIn(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	return runInEnv(ctx, dir, nil, name, args...)
}

// runInEnv runs a command in dir with the given environment (nil = inherit) and
// returns combined output for diagnostics.
func runInEnv(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.Bytes(), err
}
