// hello is a minimal Spade block fixture used by the runner's
// integration tests.  It implements the Go collection convention from
// ../../../../spec/worker.md §Execution: a single binary whose first
// argument is the block name.  Today the only block is "hello", which
// writes outputs/message/message.txt and exits 0.
//
// This binary is intentionally dependency-free so it can be built with
// plain `go build` in any temp directory.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: hello <block-name>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "hello":
		if err := runHello(); err != nil {
			fmt.Fprintln(os.Stderr, "hello:", err)
			os.Exit(1)
		}
	case "broken":
		fmt.Fprintln(os.Stderr, "intentionally broken")
		os.Exit(7)
	case "map-files":
		if err := runMapFiles(); err != nil {
			fmt.Fprintln(os.Stderr, "map-files:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown block:", os.Args[1])
		os.Exit(2)
	}
}

func runHello() error {
	outDir := filepath.Join("outputs", "message")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "message.txt"), []byte("hello from spade block\n"), 0644)
}

func runMapFiles() error {
	// Enumerate the files in inputs/source/* and write a deterministic
	// expansion manifest.
	entries, err := os.ReadDir(filepath.Join("inputs", "source"))
	if err != nil {
		return err
	}
	manifestDir := filepath.Join("outputs", "manifest")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(manifestDir, "expansion.yaml"))
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintln(f, "items:")
	for _, e := range entries {
		fmt.Fprintf(f, "  - path: inputs/source/%s\n    key: %s\n", e.Name(), e.Name())
	}
	return nil
}
