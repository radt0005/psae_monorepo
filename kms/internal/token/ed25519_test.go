package token

import (
	"crypto/ed25519"
	"testing"
	"time"

	"captoken"
)

func TestEd25519VerifierRoundTrip(t *testing.T) {
	pub, priv, _ := captoken.GenerateKey()
	tok, err := captoken.NewSigner(priv).Sign(captoken.Claims{
		UserID: "alice", InvocationID: "inv-1",
		SecretNames: []string{"db"}, Expiry: time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	v := NewEd25519Verifier([]ed25519.PublicKey{pub})
	claims, err := v.Verify(tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.UserID != "alice" || !claims.Allows("db") {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestEd25519VerifierRejectsUntrusted(t *testing.T) {
	_, priv, _ := captoken.GenerateKey()
	otherPub, _, _ := captoken.GenerateKey()
	tok, _ := captoken.NewSigner(priv).Sign(captoken.Claims{UserID: "alice"})

	v := NewEd25519Verifier([]ed25519.PublicKey{otherPub})
	if _, err := v.Verify(tok); err == nil {
		t.Fatal("expected verification to fail for an untrusted signer")
	}
}
