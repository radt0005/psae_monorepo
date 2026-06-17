package blob

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFSStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := NewFSStore(t.TempDir())
	require.NoError(t, err)

	key := "gdal/1.0.0/linux/amd64.tar.gz"
	data := []byte("hello-artifact")
	require.NoError(t, s.Put(ctx, key, bytes.NewReader(data), int64(len(data)), "application/gzip"))

	ok, err := s.Exists(ctx, key)
	require.NoError(t, err)
	require.True(t, ok)

	rc, err := s.Get(ctx, key)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	require.Equal(t, data, got)
}

func TestFSStoreCopyAndDelete(t *testing.T) {
	ctx := context.Background()
	s, _ := NewFSStore(t.TempDir())
	require.NoError(t, s.Put(ctx, "staging/a.tar.gz", bytes.NewReader([]byte("x")), 1, ""))
	require.NoError(t, s.Copy(ctx, "staging/a.tar.gz", "artifacts/a.tar.gz"))

	ok, _ := s.Exists(ctx, "artifacts/a.tar.gz")
	require.True(t, ok)

	require.NoError(t, s.Delete(ctx, "staging/a.tar.gz"))
	ok, _ = s.Exists(ctx, "staging/a.tar.gz")
	require.False(t, ok)
	require.NoError(t, s.Delete(ctx, "staging/a.tar.gz"), "delete of absent key is a no-op")
}

func TestFSStoreGetMissing(t *testing.T) {
	ctx := context.Background()
	s, _ := NewFSStore(t.TempDir())
	_, err := s.Get(ctx, "nope")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestFSStoreRejectsTraversal(t *testing.T) {
	ctx := context.Background()
	s, _ := NewFSStore(t.TempDir())
	err := s.Put(ctx, "../escape", bytes.NewReader([]byte("x")), 1, "")
	require.NoError(t, err, "cleaned to a path under root")
	// The cleaned key should not have escaped the root.
	ok, err := s.Exists(ctx, "escape")
	require.NoError(t, err)
	require.True(t, ok)
}
