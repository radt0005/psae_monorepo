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

	// filepath.Walk uses Lstat, so symlinks (including symlinks to directories)
	// surface as non-dir entries and are not descended into. Collect every
	// regular file and symlink; skip real directories (implied by their entries).
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
		// Lstat (not Stat) so a symlink is reported as a symlink rather than its
		// target: dereferencing bloats file symlinks into byte copies and makes
		// directory symlinks (e.g. a venv's lib64 -> lib, renv cache links) fail
		// with "is a directory". The worker's Unpack recreates these links.
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return "", err
		}
		rel = filepath.ToSlash(rel)

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return "", err
			}
			// Keep the link target verbatim (relative or absolute); absolute
			// targets resolve on the worker because the bundler is the worker
			// base image plus toolchains.
			hdr := &tar.Header{
				Typeflag: tar.TypeSymlink,
				Name:     rel,
				Linkname: target,
				Mode:     0o777,
				ModTime:  zeroTime, // determinism
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return "", err
			}
			continue
		}

		// Normalize modes to 0644, preserving the executable bit for binaries.
		mode := int64(0o644)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0o755
		}
		hdr := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     rel,
			Mode:     mode,
			Size:     info.Size(),
			ModTime:  zeroTime, // determinism
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
