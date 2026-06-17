// Package blob abstracts artifact object storage behind a small interface with
// two implementations: a filesystem store (tests and trivial local runs) and
// an S3-compatible store (MinIO locally, DigitalOcean Spaces in production).
package blob

import (
	"context"
	"io"
)

// Store is the object-storage contract used by the registry and builder.
type Store interface {
	// Put writes the object at key. size may be -1 if unknown.
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// Get opens the object at key for reading.
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Exists reports whether key is present.
	Exists(ctx context.Context, key string) (bool, error)
	// Delete removes key (no error if absent).
	Delete(ctx context.Context, key string) error
	// Copy duplicates src to dst within the same store.
	Copy(ctx context.Context, src, dst string) error
}
