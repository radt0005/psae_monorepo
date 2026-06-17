// Package sign implements the registry's ed25519 artifact signing and the
// trusted-public-key set served at /pubkeys, including flag-day-free key
// rotation (registry.md §6).
//
// Asymmetric signatures are mandatory: the registry holds the private key and
// workers carry only public keys, so a worker compromise cannot forge artifacts
// for other workers.
package sign

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Keypair is a base64-encoded ed25519 keypair.
type Keypair struct {
	Public  string // base64 std of the 32-byte public key
	Private string // base64 std of the 64-byte private key
}

// GenerateKeypair creates a fresh ed25519 keypair.
func GenerateKeypair() (Keypair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Keypair{}, err
	}
	return Keypair{
		Public:  base64.StdEncoding.EncodeToString(pub),
		Private: base64.StdEncoding.EncodeToString(priv),
	}, nil
}

// DecodePublic decodes a base64 public key.
func DecodePublic(b64 string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decoding public key: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key wrong size: %d", len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

// DecodePrivate decodes a base64 private key.
func DecodePrivate(b64 string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decoding private key: %w", err)
	}
	if len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("private key wrong size: %d", len(raw))
	}
	return ed25519.PrivateKey(raw), nil
}

// Sign signs data with the base64 private key, returning the raw signature.
func Sign(privateB64 string, data []byte) ([]byte, error) {
	priv, err := DecodePrivate(privateB64)
	if err != nil {
		return nil, err
	}
	return ed25519.Sign(priv, data), nil
}

// SignReader streams r fully into memory and signs it. Artifacts are modestly
// sized tarballs; ed25519 signs the whole message, not a digest.
func SignReader(privateB64 string, r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Sign(privateB64, data)
}

// Verify reports whether sig is valid for data under any of the trusted base64
// public keys (the worker accepts a list of keys during rotation — §6.1).
func Verify(trustedB64 []string, data, sig []byte) bool {
	for _, b64 := range trustedB64 {
		pub, err := DecodePublic(b64)
		if err != nil {
			continue
		}
		if ed25519.Verify(pub, data, sig) {
			return true
		}
	}
	return false
}

// ErrNoActiveKey is returned when signing is attempted with no active key.
var ErrNoActiveKey = errors.New("sign: no active signing key")
