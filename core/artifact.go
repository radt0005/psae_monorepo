package core

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Unpack extracts a gzip-compressed tar stream into destDir, recreating regular
// files (preserving the executable bit) and symlinks (tar.TypeSymlink). It is the
// inverse of the registry packager (registry PackageTarGz), which emits symlinks
// verbatim so relocatable Python venvs and R libraries survive the round trip.
//
// Entry paths are validated to stay within destDir: absolute names, or names that
// escape via "..", are rejected so a malicious artifact cannot write outside the
// install directory. Symlink *targets* are not restricted — they legitimately
// point at absolute system paths (e.g. a venv's python -> /usr/bin/python3) that
// resolve on the worker because the bundler is the worker base image plus
// toolchains.
func Unpack(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("opening gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	cleanDest := filepath.Clean(destDir)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		target, err := safeJoin(cleanDest, hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			// Remove any existing entry so re-unpacking is idempotent.
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			if hdr.Mode&0o111 != 0 {
				mode = 0o755
			}
			f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil { //nolint:gosec // sizes are bounded by trusted, verified artifacts
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		default:
			// Skip other entry types (hardlinks, devices) — the packager never
			// emits them.
		}
	}
	return nil
}

// safeJoin joins name onto dir, rejecting absolute paths and any name that would
// escape dir via "..". Returns the cleaned absolute target.
func safeJoin(dir, name string) (string, error) {
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("unpack: absolute path in artifact: %q", name)
	}
	target := filepath.Join(dir, name)
	if target != dir && !strings.HasPrefix(target, dir+string(os.PathSeparator)) {
		return "", fmt.Errorf("unpack: path escapes destination: %q", name)
	}
	return target, nil
}

// VerifySignature reports whether sig is a valid ed25519 signature over data
// under any of the trusted base64-encoded public keys. Accepting a *list* of keys
// makes key rotation flag-day-free: during a rotation both the old and new keys
// are trusted (registry.md §6.1). Mirrors registry sign.Verify on the producer
// side, but lives in core so the worker can verify without importing the registry.
func VerifySignature(trustedPubKeysB64 []string, data, sig []byte) bool {
	for _, b64 := range trustedPubKeysB64 {
		raw, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			continue
		}
		if ed25519.Verify(ed25519.PublicKey(raw), data, sig) {
			return true
		}
	}
	return false
}

// HashMatches reports whether the sha256 of data equals the given hex digest
// (case-insensitive), the value the registry records in artifact metadata.
func HashMatches(data []byte, hexDigest string) bool {
	sum := sha256.Sum256(data)
	return strings.EqualFold(hex.EncodeToString(sum[:]), strings.TrimSpace(hexDigest))
}
