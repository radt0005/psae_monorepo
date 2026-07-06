package builder

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// RustBuilder builds a Rust collection: `cargo build --release` into a single
// binary with subcommands, packaged alongside the blocks/*.yaml manifests
// (registry.md §5, "Rust / Go / TypeScript: a single binary plus the
// blocks/*.yaml manifests"). Native dependencies (GDAL, PROJ, Arrow) link
// dynamically against the system libraries present in the worker runtime image;
// because the bundler image is the worker base image plus toolchains, those
// shared-object versions match at execution time.
type RustBuilder struct{}

func (RustBuilder) Build(ctx context.Context, srcDir, collection, version string) (string, error) {
	artifactDir, err := os.MkdirTemp("", "spade-artifact-*")
	if err != nil {
		return "", err
	}

	// --message-format=json makes cargo emit one JSON record per line; the
	// records with a non-null "executable" field name the produced binaries.
	cmd := exec.CommandContext(ctx, "cargo", "build", "--release", "--message-format=json")
	cmd.Dir = srcDir
	cmd.Env = append(os.Environ(), "CARGO_TERM_COLOR=never")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(artifactDir)
		return "", fmt.Errorf("cargo build failed: %v\n%s", err, stderr.String())
	}

	binPath, err := rustExecutable(&stdout, collection)
	if err != nil {
		os.RemoveAll(artifactDir)
		return "", err
	}

	// Copy the produced binary to <artifactDir>/<collection> so the worker can
	// invoke it as `<dir>/<collection> <block>` (core.ResolveEntrypoint).
	if err := copyFileMode(binPath, filepath.Join(artifactDir, collection), 0o755); err != nil {
		os.RemoveAll(artifactDir)
		return "", fmt.Errorf("staging rust binary: %w", err)
	}
	if err := copyBlocksDir(srcDir, artifactDir); err != nil {
		os.RemoveAll(artifactDir)
		return "", err
	}
	return artifactDir, nil
}

// rustExecutable parses cargo's JSON message stream and returns the path of the
// collection binary. A Spade collection is a single binary with subcommands, so
// we expect exactly one executable; if cargo produced several, we prefer the one
// whose target name matches the collection and error if that is ambiguous.
func rustExecutable(jsonStream io.Reader, collection string) (string, error) {
	type artifactMsg struct {
		Reason     string `json:"reason"`
		Executable string `json:"executable"`
		Target     struct {
			Name string `json:"name"`
		} `json:"target"`
	}

	var executables []artifactMsg
	sc := bufio.NewScanner(jsonStream)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		var m artifactMsg
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			continue // non-artifact diagnostic lines
		}
		if m.Reason == "compiler-artifact" && m.Executable != "" {
			executables = append(executables, m)
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("reading cargo output: %w", err)
	}

	switch len(executables) {
	case 0:
		return "", fmt.Errorf("cargo produced no executable; a Spade collection must build a binary")
	case 1:
		return executables[0].Executable, nil
	default:
		for _, m := range executables {
			if m.Target.Name == collection {
				return m.Executable, nil
			}
		}
		return "", fmt.Errorf("cargo produced %d executables and none is named %q; a collection must be a single binary", len(executables), collection)
	}
}
