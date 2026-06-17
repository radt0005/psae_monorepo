package blob

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestS3StoreRoundTrip exercises the S3 backend against a live S3-compatible
// endpoint (MinIO). It is skipped unless S3_ENDPOINT and S3_BUCKET are set, so
// the default `go test ./...` run needs no external services.
//
//	S3_ENDPOINT=http://localhost:9000 S3_BUCKET=spade-artifacts \
//	S3_ACCESS_KEY_ID=spade S3_SECRET_ACCESS_KEY=spadespade go test ./internal/blob/
func TestS3StoreRoundTrip(t *testing.T) {
	endpoint := os.Getenv("S3_ENDPOINT")
	bucket := os.Getenv("S3_BUCKET")
	if endpoint == "" || bucket == "" {
		t.Skip("set S3_ENDPOINT and S3_BUCKET to run the live S3 test")
	}
	ctx := context.Background()
	s, err := NewS3Store(ctx, S3Options{
		Endpoint:     endpoint,
		Region:       envOr("S3_REGION", "us-east-1"),
		Bucket:       bucket,
		AccessKey:    os.Getenv("S3_ACCESS_KEY_ID"),
		SecretKey:    os.Getenv("S3_SECRET_ACCESS_KEY"),
		UsePathStyle: true,
	})
	require.NoError(t, err)

	key := "test/blob_test/" + t.Name() + ".bin"
	data := []byte("s3-round-trip")
	require.NoError(t, s.Put(ctx, key, bytes.NewReader(data), int64(len(data)), "application/octet-stream"))
	t.Cleanup(func() { _ = s.Delete(ctx, key) })

	ok, err := s.Exists(ctx, key)
	require.NoError(t, err)
	require.True(t, ok)

	rc, err := s.Get(ctx, key)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	require.Equal(t, data, got)

	dst := key + ".copy"
	require.NoError(t, s.Copy(ctx, key, dst))
	t.Cleanup(func() { _ = s.Delete(ctx, dst) })
	ok, err = s.Exists(ctx, dst)
	require.NoError(t, err)
	require.True(t, ok)

	_, err = s.Get(ctx, "test/blob_test/missing-"+t.Name())
	require.ErrorIs(t, err, os.ErrNotExist)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
