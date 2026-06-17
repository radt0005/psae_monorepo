package builder

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// zeroTime is the fixed mtime stamped into every tar entry for determinism.
var zeroTime = time.Unix(0, 0).UTC()

// PackageTarGz writes a deterministic gzip-compressed tarball of dir's contents
// to w and returns the sha256 hex of the produced bytes. Determinism (sorted
// entries, zeroed mtimes) keeps the content hash stable for caching and for the
// signature the control plane produces over these exact bytes.
//
// It returns the hash computed over what was written to w.
func PackageTarGz(dir string, w io.Writer) (string, error) {
	hasher := sha256.New()
	mw := io.MultiWriter(w, hasher)

	gz := gzip.NewWriter(mw)
	tw := tar.NewWriter(gz)

	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return "", err
		}
		rel = filepath.ToSlash(rel)

		// Normalize modes to 0644, preserving the executable bit for binaries.
		mode := int64(0o644)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0o755
		}
		hdr := &tar.Header{
			Name:    rel,
			Mode:    mode,
			Size:    info.Size(),
			ModTime: zeroTime, // determinism
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return "", err
		}
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(tw, f); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
	}
	if err := tw.Close(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// HashReader returns the sha256 hex of r's full contents.
func HashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
