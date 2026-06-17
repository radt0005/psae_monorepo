package builder

import (
	"bytes"
	"context"
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

func TestBuilderForLanguages(t *testing.T) {
	b, err := BuilderFor("go")
	require.NoError(t, err)
	require.IsType(t, GoBuilder{}, b)

	for _, lang := range []string{"python", "r", "rust", "typescript"} {
		b, err := BuilderFor(lang)
		require.NoError(t, err)
		_, berr := b.Build(context.Background(), t.TempDir(), "c", "1.0.0")
		require.Error(t, berr, "%s builder is a stub", lang)
	}

	_, err = BuilderFor("cobol")
	require.Error(t, err)
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
