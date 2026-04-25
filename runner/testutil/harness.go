// Package testutil provides shared fixtures and helpers used across the
// spade_runner test suites.  The primary capability is InstallHelloFixture,
// which builds the Go block fixture at testutil/fixtures/hello-go and
// registers it in a fresh core.BlockRegistry.
package testutil

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"core"
)

// FixtureRoot returns the absolute path to the directory containing the
// hello-go test fixture.  It resolves to <runner>/testutil/fixtures.
func FixtureRoot() string {
	// Compute relative to this source file so tests work regardless of
	// where `go test` is invoked.
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "fixtures")
}

// IsolateFriendlyTempDir returns a temp directory under $HOME (or
// $SPADE_TEST_ROOT if set) rather than /tmp.  Some isolate hosts
// cannot mount /tmp paths because the remapped sandbox user lacks
// permission on tmpfs; using $HOME sidesteps that.  The dir is chmod
// 0777 so the remapped sandbox uid can write inside it, and t.Cleanup
// removes it at test-end.
func IsolateFriendlyTempDir(t *testing.T) string {
	t.Helper()
	root := os.Getenv("SPADE_TEST_ROOT")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("UserHomeDir: %v", err)
		}
		root = filepath.Join(home, ".spade-integration-tests")
	}
	if err := os.MkdirAll(root, 0777); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}
	dir, err := os.MkdirTemp(root, t.Name()+"-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// InstalledFixture is returned by InstallHelloFixture.
type InstalledFixture struct {
	InstalledPath string
	CollectionBin string // absolute path to the built binary
}

// InstallHelloFixture copies the hello-go fixture into a temp install
// directory, builds it with `go build`, computes a content hash, and
// registers all three blocks (hello, broken, map-files) in the given
// registry.
//
// Returns the InstalledFixture.  Test cleanup (t.Cleanup) removes the
// temp install dir.
//
// The fixture is installed under $HOME (or $SPADE_TEST_ROOT) rather
// than /tmp.  Some isolate hosts cannot mount /tmp paths because the
// remapped sandbox user lacks access to tmpfs dirs, which would break
// integration tests that bind the install dir into the sandbox.
func InstallHelloFixture(t *testing.T, reg *core.BlockRegistry) InstalledFixture {
	t.Helper()

	src := filepath.Join(FixtureRoot(), "hello-go")
	dst := IsolateFriendlyTempDir(t)

	// Copy the fixture into the temp install dir (so the build output
	// lives next to the manifests — matching the installed layout).
	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	// Build the binary.  The collection name is "hello" — matching go.mod.
	binPath := filepath.Join(dst, "hello")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = dst
	build.Env = append(os.Environ(), "GOFLAGS=-buildvcs=false")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build fixture: %v\n%s", err, out)
	}

	hash, err := core.ComputeContentHash(dst)
	if err != nil {
		t.Fatalf("hashing install: %v", err)
	}

	for _, blockName := range []string{"hello", "broken", "map-files"} {
		manifestPath := filepath.Join(dst, "blocks", blockName+".yaml")
		m, err := core.LoadBlockManifest(manifestPath)
		if err != nil {
			t.Fatalf("loading manifest %s: %v", manifestPath, err)
		}
		entry := core.BlockRegistryEntry{
			CollectionName:    "hello",
			CollectionVersion: m.Version,
			BlockName:         blockName,
			BlockID:           m.ID,
			Language:          string(core.CollectionLanguageGo),
			Entrypoint:        blockName,
			InstalledPath:     dst,
			ContentHash:       hash,
			Kind:              string(m.Kind),
			Network:           m.Network,
		}
		if err := reg.RegisterBlock(entry); err != nil {
			t.Fatalf("register block %s: %v", blockName, err)
		}
	}

	return InstalledFixture{InstalledPath: dst, CollectionBin: binPath}
}

// SilentLogger returns a slog.Logger that discards all output.
// Useful to keep test output readable when exercising the worker loop.
func SilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// copyTree recursively copies src to dst.  Files preserve their mode.
func copyTree(src, dst string) error {
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
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
