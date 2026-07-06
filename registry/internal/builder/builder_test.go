package builder

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"spade_registry/internal/testutil"
)

func TestNoopScreenerPasses(t *testing.T) {
	res, err := NoopScreener{}.Screen(context.Background(), t.TempDir())
	require.NoError(t, err)
	require.True(t, res.Passed)
	require.Equal(t, "noop", res.ScreenerName)
}

func TestPackageTarGzDeterministic(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "blocks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "blocks", "b.yaml"), []byte("id: x"), 0o644))

	var buf1, buf2 bytes.Buffer
	h1, err := PackageTarGz(dir, &buf1)
	require.NoError(t, err)
	h2, err := PackageTarGz(dir, &buf2)
	require.NoError(t, err)

	require.Equal(t, h1, h2, "packaging is deterministic")
	require.Equal(t, buf1.Bytes(), buf2.Bytes())

	// The hash matches a hash of the produced bytes.
	streamHash, err := HashReader(bytes.NewReader(buf1.Bytes()))
	require.NoError(t, err)
	require.Equal(t, h1, streamHash)
}

func TestPackageTarGzPreservesSymlinks(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "real.txt"), []byte("hi"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "lib"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lib", "a.so"), []byte("x"), 0o644))
	// A file symlink and a directory symlink (the two cases that broke the old
	// dereferencing packager: bloat, and "is a directory").
	require.NoError(t, os.Symlink("real.txt", filepath.Join(dir, "link.txt")))
	require.NoError(t, os.Symlink("lib", filepath.Join(dir, "lib64")))

	var buf bytes.Buffer
	_, err := PackageTarGz(dir, &buf)
	require.NoError(t, err)

	// Inspect the tar: the symlinks are TypeSymlink with their targets, not copies.
	gz, err := gzip.NewReader(&buf)
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	links := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if hdr.Typeflag == tar.TypeSymlink {
			links[hdr.Name] = hdr.Linkname
		}
	}
	require.Equal(t, "real.txt", links["link.txt"], "file symlink preserved")
	require.Equal(t, "lib", links["lib64"], "directory symlink preserved")
}

func TestBuilderForLanguages(t *testing.T) {
	// All five supported languages have real builders.
	realCases := map[string]any{
		"go":         GoBuilder{},
		"rust":       RustBuilder{},
		"typescript": BunBuilder{},
		"python":     PythonBuilder{},
		"r":          RBuilder{},
	}
	for lang, want := range realCases {
		b, err := BuilderFor(lang)
		require.NoError(t, err)
		require.IsType(t, want, b, "%s builder", lang)
	}

	_, err := BuilderFor("cobol")
	require.Error(t, err)
}

func TestRustBuilderBuildsFixture(t *testing.T) {
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo toolchain not available")
	}
	col := testutil.NewRustCollectionRepo(t)

	artifactDir, err := RustBuilder{}.Build(context.Background(), col.Dir, col.Collection, col.Version)
	require.NoError(t, err)
	defer os.RemoveAll(artifactDir)

	binPath := filepath.Join(artifactDir, col.Collection)
	info, err := os.Stat(binPath)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&0o111, "binary is executable")

	_, err = os.Stat(filepath.Join(artifactDir, "blocks", "greet.yaml"))
	require.NoError(t, err)

	out, err := exec.Command(binPath, "greet").CombinedOutput()
	require.NoError(t, err, "running built binary: %s", out)
	require.Contains(t, string(out), "hello from the fixture")
}

func TestBunBuilderBuildsFixture(t *testing.T) {
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun toolchain not available")
	}
	col := testutil.NewBunCollectionRepo(t)

	artifactDir, err := BunBuilder{}.Build(context.Background(), col.Dir, col.Collection, col.Version)
	require.NoError(t, err)
	defer os.RemoveAll(artifactDir)

	binPath := filepath.Join(artifactDir, col.Collection)
	info, err := os.Stat(binPath)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&0o111, "compiled executable is executable")

	_, err = os.Stat(filepath.Join(artifactDir, "blocks", "greet.yaml"))
	require.NoError(t, err)

	out, err := exec.Command(binPath, "greet").CombinedOutput()
	require.NoError(t, err, "running compiled binary: %s", out)
	require.Contains(t, string(out), "hello from the fixture")
}

func TestGoBuilderBuildsFixture(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	col := testutil.NewGoCollectionRepo(t)

	artifactDir, err := GoBuilder{}.Build(context.Background(), col.Dir, col.Collection, col.Version)
	require.NoError(t, err)
	defer os.RemoveAll(artifactDir)

	// The single collection binary exists and is executable.
	binPath := filepath.Join(artifactDir, col.Collection)
	info, err := os.Stat(binPath)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&0o111, "binary is executable")

	// The blocks manifests were packaged alongside the binary.
	_, err = os.Stat(filepath.Join(artifactDir, "blocks", "greet.yaml"))
	require.NoError(t, err)

	// The built binary runs and dispatches its subcommand.
	out, err := exec.Command(binPath, "greet").CombinedOutput()
	require.NoError(t, err, "running built binary: %s", out)
	require.Contains(t, string(out), "hello from the fixture")
}

func TestPythonBuilderBuildsRelocatableVenv(t *testing.T) {
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv toolchain not available")
	}
	col := testutil.NewPythonCollectionRepo(t)

	artifactDir, err := PythonBuilder{}.Build(context.Background(), col.Dir, col.Collection, col.Version)
	require.NoError(t, err)
	defer os.RemoveAll(artifactDir)

	// The venv and the shipped source both exist.
	_, err = os.Stat(filepath.Join(artifactDir, ".venv", "bin", "python"))
	require.NoError(t, err, "venv interpreter present")
	_, err = os.Stat(filepath.Join(artifactDir, "blocks", "greet.yaml"))
	require.NoError(t, err)

	// Relocate the artifact, then run the block exactly as the worker would:
	// `uv run --project <dir> --no-sync -m <module>`. This proves the venv is
	// relocatable and the collection is importable offline (non-editable install).
	moved := filepath.Join(t.TempDir(), "installed")
	require.NoError(t, os.Rename(artifactDir, moved))
	defer os.RemoveAll(moved)

	cmd := exec.Command("uv", "run", "--project", moved, "--no-sync", "-m", "hello.greet")
	cmd.Env = append(os.Environ(), "UV_OFFLINE=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "running relocated block: %s", out)
	require.Contains(t, string(out), "hello from the fixture")
}

func TestRBuilderBundlesLibrary(t *testing.T) {
	if _, err := exec.LookPath("Rscript"); err != nil {
		t.Skip("Rscript not available")
	}
	col := testutil.NewRCollectionRepo(t)

	artifactDir, err := RBuilder{}.Build(context.Background(), col.Dir, col.Collection, col.Version)
	require.NoError(t, err)
	defer os.RemoveAll(artifactDir)

	// setup.R ran with R_LIBS_USER pointed inside the artifact: the marker it
	// wrote proves the builder captured installs into the shipped library.
	_, err = os.Stat(filepath.Join(artifactDir, "renv", "library", "setup-marker"))
	require.NoError(t, err, "setup.R installed into the artifact library")

	// The block script and manifest ship in the artifact.
	scriptPath := filepath.Join(artifactDir, "R", "greet.R")
	_, err = os.Stat(scriptPath)
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(artifactDir, "blocks", "greet.yaml"))
	require.NoError(t, err)

	// Relocate, then run the block as the worker would (`Rscript <dir>/R/x.R`).
	moved := filepath.Join(t.TempDir(), "installed")
	require.NoError(t, os.Rename(artifactDir, moved))
	defer os.RemoveAll(moved)

	out, err := exec.Command("Rscript", filepath.Join(moved, "R", "greet.R")).CombinedOutput()
	require.NoError(t, err, "running relocated block: %s", out)
	require.Contains(t, string(out), "hello from the fixture")
}

func TestRBuilderPakBundlesLibrary(t *testing.T) {
	if _, err := exec.LookPath("Rscript"); err != nil {
		t.Skip("Rscript not available")
	}
	if err := exec.Command("Rscript", "-e",
		"if (!requireNamespace('pak', quietly=TRUE)) quit(status=1)").Run(); err != nil {
		t.Skip("pak not available")
	}
	col := testutil.NewRPakCollectionRepo(t)

	artifactDir, err := RBuilder{}.Build(context.Background(), col.Dir, col.Collection, col.Version)
	require.NoError(t, err)
	defer os.RemoveAll(artifactDir)

	libDir := filepath.Join(artifactDir, "renv", "library")

	// The DESCRIPTION dep landed in the artifact library.
	_, err = os.Stat(filepath.Join(libDir, "jsonlite"))
	require.NoError(t, err, "jsonlite installed into the artifact library")

	// R0-a: pak's `_cache` staging dir was cleaned out of the shipped library.
	_, err = os.Stat(filepath.Join(libDir, "_cache"))
	require.True(t, os.IsNotExist(err), "pak _cache removed from the library")

	// The library holds real files, not symlinks into a cache: this is what lets
	// PackageTarGz (which copies file bytes) ship it, and what pak buys us over an
	// renv cache. Fail if any symlink slipped in.
	require.NoError(t, filepath.Walk(libDir, func(path string, info os.FileInfo, err error) error {
		require.NoError(t, err)
		require.Zero(t, info.Mode()&os.ModeSymlink, "unexpected symlink in library: %s", path)
		return nil
	}))

	// R0-b: the committed pak lockfile records a relative `deps::.` ref, not an
	// absolute build path, so it stays portable.
	lock, err := os.ReadFile(filepath.Join(artifactDir, "pkg.lock"))
	require.NoError(t, err)
	require.Contains(t, string(lock), `"deps::."`, "pkg.lock top-level ref is relative")
	require.NotContains(t, string(lock), artifactDir, "pkg.lock leaks the build path")

	// Package the artifact and unpack it to a *different* path, then run the block
	// exactly as the worker would, pointing R's search path at the shipped library.
	// This is the real tar round-trip (notes.md §C5), proving the compiled .so
	// relocates and loads offline — not the os.Rename shortcut the setup.R test uses.
	tarPath := filepath.Join(t.TempDir(), "artifact.tar.gz")
	f, err := os.Create(tarPath)
	require.NoError(t, err)
	_, err = PackageTarGz(artifactDir, f)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	dest := t.TempDir()
	out, err := exec.Command("tar", "xzf", tarPath, "-C", dest).CombinedOutput()
	require.NoError(t, err, "untar: %s", out)

	movedLib := filepath.Join(dest, "renv", "library")
	cmd := exec.Command("Rscript", filepath.Join(dest, "R", "greet.R"))
	cmd.Env = append(os.Environ(), "R_LIBS="+movedLib, "R_LIBS_USER="+movedLib)
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "running relocated block: %s", out)
	require.Contains(t, string(out), "hello from the fixture")
}

func TestGitClonerCheckoutSHA(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	col := testutil.NewGoCollectionRepo(t)

	dir, cleanup, err := GitCloner{}.Clone(context.Background(), col.RepoURL, col.CommitSHA)
	require.NoError(t, err)
	defer cleanup()

	// The checked-out tree contains the committed files.
	_, err = os.Stat(filepath.Join(dir, "go.mod"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "blocks", "greet.yaml"))
	require.NoError(t, err)
}

func TestCollectBlocks(t *testing.T) {
	col := testutil.NewGoCollectionRepo(t)
	blocks, err := collectBlocks(col.Dir)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, "hello.greet", blocks[0].ID)
	require.Equal(t, "greet", blocks[0].Name)
	require.Contains(t, blocks[0].Inputs, "name")
}
