package envelope

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func randKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, keySize)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSealOpenRoundTrip(t *testing.T) {
	ks, err := NewKeySet(map[string][]byte{"v1": randKey(t)}, "v1")
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("postgres://user:pw@host/db")

	sealed, err := ks.Seal(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed.Ciphertext, plaintext) {
		t.Fatal("ciphertext must not contain the plaintext")
	}
	if sealed.KEKID != "v1" {
		t.Fatalf("KEKID = %q, want v1", sealed.KEKID)
	}

	got, err := ks.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("Open = %q, want %q", got, plaintext)
	}
}

func TestOpenWithDifferentKeyFails(t *testing.T) {
	ks1, _ := NewKeySet(map[string][]byte{"v1": randKey(t)}, "v1")
	sealed, _ := ks1.Seal([]byte("secret"))

	// A different keyset with the same id but different bytes cannot open it.
	ks2, _ := NewKeySet(map[string][]byte{"v1": randKey(t)}, "v1")
	if _, err := ks2.Open(sealed); err == nil {
		t.Fatal("expected Open to fail under a different KEK")
	}
}

func TestUnknownKEKID(t *testing.T) {
	ks, _ := NewKeySet(map[string][]byte{"v1": randKey(t)}, "v1")
	sealed, _ := ks.Seal([]byte("x"))
	sealed.KEKID = "vX"
	if _, err := ks.Open(sealed); err == nil {
		t.Fatal("expected error for unknown KEK id")
	}
}

func TestRotationOldKeyStillOpens(t *testing.T) {
	k1 := randKey(t)
	ksOld, _ := NewKeySet(map[string][]byte{"v1": k1}, "v1")
	sealed, _ := ksOld.Seal([]byte("legacy"))

	// After adding v2 as active, old v1-wrapped secrets still open, and new
	// seals use v2.
	ksNew, _ := NewKeySet(map[string][]byte{"v1": k1, "v2": randKey(t)}, "v2")
	got, err := ksNew.Open(sealed)
	if err != nil {
		t.Fatalf("rotation open: %v", err)
	}
	if string(got) != "legacy" {
		t.Fatalf("got %q", got)
	}
	fresh, _ := ksNew.Seal([]byte("fresh"))
	if fresh.KEKID != "v2" {
		t.Fatalf("new seal should use active KEK v2, got %q", fresh.KEKID)
	}
}

func TestNewKeySetValidation(t *testing.T) {
	if _, err := NewKeySet(map[string][]byte{}, "v1"); err == nil {
		t.Fatal("expected error for empty key set")
	}
	if _, err := NewKeySet(map[string][]byte{"v1": make([]byte, 16)}, "v1"); err == nil {
		t.Fatal("expected error for wrong key size")
	}
	if _, err := NewKeySet(map[string][]byte{"v1": make([]byte, keySize)}, "v2"); err == nil {
		t.Fatal("expected error for missing active id")
	}
}

func TestParseKeys(t *testing.T) {
	spec := "v1:" + base64.StdEncoding.EncodeToString(make([]byte, keySize))
	keys, err := ParseKeys(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys["v1"]) != keySize {
		t.Fatalf("parsed key size = %d", len(keys["v1"]))
	}
	if _, err := ParseKeys("bad-entry-no-colon"); err == nil {
		t.Fatal("expected malformed-entry error")
	}
}
