package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// writeTarGz builds a gzip tarball from the given entries for Unpack tests.
type tarEntry struct {
	name     string
	typeflag byte
	body     string
	linkname string
	mode     int64
}

func writeTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Typeflag: e.typeflag, Name: e.name, Linkname: e.linkname, Mode: e.mode}
		if e.typeflag == tar.TypeReg {
			hdr.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if e.typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUnpackFilesAndSymlinks(t *testing.T) {
	data := writeTarGz(t, []tarEntry{
		{name: "real.txt", typeflag: tar.TypeReg, body: "hello", mode: 0o644},
		{name: "bin/tool", typeflag: tar.TypeReg, body: "#!/bin/sh\n", mode: 0o755},
		{name: "link.txt", typeflag: tar.TypeSymlink, linkname: "real.txt"},
		{name: "lib64", typeflag: tar.TypeSymlink, linkname: "lib"}, // dir symlink
	})

	dest := t.TempDir()
	if err := Unpack(bytes.NewReader(data), dest); err != nil {
		t.Fatal(err)
	}

	// Regular file content and the executable bit round-trip.
	got, err := os.ReadFile(filepath.Join(dest, "real.txt"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("real.txt = %q, %v", got, err)
	}
	fi, err := os.Stat(filepath.Join(dest, "bin", "tool"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o111 == 0 {
		t.Errorf("executable bit not preserved: %v", fi.Mode())
	}

	// Symlinks are recreated as symlinks with their targets.
	tgt, err := os.Readlink(filepath.Join(dest, "link.txt"))
	if err != nil || tgt != "real.txt" {
		t.Errorf("link.txt -> %q, %v", tgt, err)
	}
	tgt, err = os.Readlink(filepath.Join(dest, "lib64"))
	if err != nil || tgt != "lib" {
		t.Errorf("lib64 -> %q, %v", tgt, err)
	}
}

func TestUnpackRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../escape", "/etc/passwd", "a/../../b"} {
		data := writeTarGz(t, []tarEntry{{name: name, typeflag: tar.TypeReg, body: "x", mode: 0o644}})
		if err := Unpack(bytes.NewReader(data), t.TempDir()); err == nil {
			t.Errorf("expected rejection for %q", name)
		}
	}
}

func TestVerifySignatureAndRotation(t *testing.T) {
	pub1, priv1, _ := ed25519.GenerateKey(rand.Reader)
	pub2, priv2, _ := ed25519.GenerateKey(rand.Reader)
	b64 := func(k []byte) string { return base64.StdEncoding.EncodeToString(k) }

	data := []byte("artifact bytes")
	sig1 := ed25519.Sign(priv1, data)
	sig2 := ed25519.Sign(priv2, data)
	trusted := []string{b64(pub1), b64(pub2)} // old + new during rotation

	if !VerifySignature(trusted, data, sig1) {
		t.Error("sig from key1 should verify")
	}
	if !VerifySignature(trusted, data, sig2) {
		t.Error("sig from key2 should verify (rotation)")
	}
	// Tampered data fails.
	if VerifySignature(trusted, []byte("tampered"), sig1) {
		t.Error("tampered data must not verify")
	}
	// Untrusted key fails.
	_, privX, _ := ed25519.GenerateKey(rand.Reader)
	if VerifySignature(trusted, data, ed25519.Sign(privX, data)) {
		t.Error("untrusted key must not verify")
	}
}

func TestHashMatches(t *testing.T) {
	data := []byte("some artifact")
	sum := sha256.Sum256(data)
	if !HashMatches(data, hex.EncodeToString(sum[:])) {
		t.Error("matching hash should pass")
	}
	if !HashMatches(data, "  "+hex.EncodeToString(sum[:])+"  ") {
		t.Error("surrounding whitespace should be tolerated")
	}
	if HashMatches(data, "deadbeef") {
		t.Error("wrong hash should fail")
	}
}
