package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FSStore is a filesystem-backed Store rooted at a directory. Object keys map to
// relative paths under root. Used by tests and trivial local deployments.
type FSStore struct {
	root string
}

// NewFSStore creates a filesystem store rooted at root (created if absent).
func NewFSStore(root string) (*FSStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("creating blob root: %w", err)
	}
	return &FSStore{root: root}, nil
}

// path resolves key to a path under root, rejecting traversal escapes.
func (s *FSStore) path(key string) (string, error) {
	clean := filepath.Clean("/" + strings.TrimPrefix(key, "/"))
	p := filepath.Join(s.root, clean)
	if !strings.HasPrefix(p, filepath.Clean(s.root)+string(os.PathSeparator)) && p != filepath.Clean(s.root) {
		return "", fmt.Errorf("invalid key %q", key)
	}
	return p, nil
}

func (s *FSStore) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return f.Sync()
}

func (s *FSStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	p, err := s.path(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("blob %q: %w", key, os.ErrNotExist)
	}
	return f, err
}

func (s *FSStore) Exists(ctx context.Context, key string) (bool, error) {
	p, err := s.path(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *FSStore) Delete(ctx context.Context, key string) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *FSStore) Copy(ctx context.Context, src, dst string) error {
	rc, err := s.Get(ctx, src)
	if err != nil {
		return err
	}
	defer rc.Close()
	return s.Put(ctx, dst, rc, -1, "")
}
