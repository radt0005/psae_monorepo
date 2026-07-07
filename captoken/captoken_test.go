package captoken

import (
	"crypto/ed25519"
	"testing"
	"time"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	pub, priv, _ := GenerateKey()
	s := NewSigner(priv)

	claims := Claims{
		UserID: "alice", InvocationID: "inv-1",
		SecretNames: []string{"prod-dsn", "api-key"},
		Expiry:      time.Now().Add(time.Minute).Truncate(time.Second),
	}
	tok, err := s.Sign(claims)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Verify(tok, []ed25519.PublicKey{pub})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.UserID != "alice" || got.InvocationID != "inv-1" {
		t.Fatalf("claims mismatch: %+v", got)
	}
	if len(got.SecretNames) != 2 || got.SecretNames[0] != "prod-dsn" {
		t.Fatalf("names mismatch: %v", got.SecretNames)
	}
	if !got.Expiry.Equal(claims.Expiry) {
		t.Fatalf("expiry mismatch: %v vs %v", got.Expiry, claims.Expiry)
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	_, priv, _ := GenerateKey()
	otherPub, _, _ := GenerateKey()
	tok, _ := NewSigner(priv).Sign(Claims{UserID: "alice"})
	if _, err := Verify(tok, []ed25519.PublicKey{otherPub}); err == nil {
		t.Fatal("expected verification to fail under the wrong key")
	}
}

func TestVerifyAcceptsAnyTrustedKey(t *testing.T) {
	// Rotation: a token signed by the old key still verifies while both keys
	// are trusted.
	oldPub, oldPriv, _ := GenerateKey()
	newPub, _, _ := GenerateKey()
	tok, _ := NewSigner(oldPriv).Sign(Claims{UserID: "alice"})
	if _, err := Verify(tok, []ed25519.PublicKey{newPub, oldPub}); err != nil {
		t.Fatalf("token signed by a trusted (old) key should verify: %v", err)
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	pub, priv, _ := GenerateKey()
	tok, _ := NewSigner(priv).Sign(Claims{UserID: "alice", SecretNames: []string{"db"}})
	// Flip a character in the payload segment.
	b := []byte(tok)
	b[0] ^= 0x01
	if _, err := Verify(string(b), []ed25519.PublicKey{pub}); err == nil {
		t.Fatal("expected verification to fail on a tampered payload")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	pub, _, _ := GenerateKey()
	for _, bad := range []string{"", "no-dot", "a.b.c", "!!!.###"} {
		if _, err := Verify(bad, []ed25519.PublicKey{pub}); err == nil {
			t.Fatalf("expected error for malformed token %q", bad)
		}
	}
}

func TestKeyEncodeParse(t *testing.T) {
	pub, priv, _ := GenerateKey()
	gotPriv, err := ParsePrivateKey(EncodeKey(priv))
	if err != nil {
		t.Fatal(err)
	}
	if !gotPriv.Equal(priv) {
		t.Fatal("private key round-trip mismatch")
	}
	keys, err := ParsePublicKeys(EncodeKey(pub))
	if err != nil || len(keys) != 1 || !keys[0].Equal(pub) {
		t.Fatalf("public key round-trip: %v %v", keys, err)
	}
}
