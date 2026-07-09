// Package captoken implements the capability token shared between the scheduler
// (which signs) and the KMS (which verifies) — spec/secrets.md §6. A token
// grants read access to a specific set of secrets, for one invocation and
// owner, until it expires.
//
// The token is `base64url(payload) . base64url(signature)` where payload is the
// JSON-encoded Claims and signature is ed25519 over the payload bytes. The
// package depends only on the standard library so both modules can import it
// without pulling heavy dependencies.
package captoken

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Claims is the capability a token asserts.
type Claims struct {
	UserID       string
	InvocationID string
	SecretNames  []string
	Expiry       time.Time
}

// wire is the on-the-wire JSON form of Claims (compact, stable field names).
type wire struct {
	UID   string   `json:"uid"`
	Inv   string   `json:"inv"`
	Names []string `json:"names"`
	Exp   int64    `json:"exp"` // Unix seconds
}

func (c Claims) toWire() wire {
	var exp int64
	if !c.Expiry.IsZero() {
		exp = c.Expiry.Unix()
	}
	return wire{UID: c.UserID, Inv: c.InvocationID, Names: c.SecretNames, Exp: exp}
}

func (w wire) toClaims() Claims {
	var exp time.Time
	if w.Exp != 0 {
		exp = time.Unix(w.Exp, 0)
	}
	return Claims{UserID: w.UID, InvocationID: w.Inv, SecretNames: w.Names, Expiry: exp}
}

var b64 = base64.RawURLEncoding

// Signer signs claims with an ed25519 private key (held by the scheduler).
type Signer struct {
	priv ed25519.PrivateKey
}

// NewSigner wraps an ed25519 private key.
func NewSigner(priv ed25519.PrivateKey) *Signer { return &Signer{priv: priv} }

// Sign encodes and signs the claims, returning the token string.
func (s *Signer) Sign(c Claims) (string, error) {
	payload, err := json.Marshal(c.toWire())
	if err != nil {
		return "", fmt.Errorf("marshaling claims: %w", err)
	}
	sig := ed25519.Sign(s.priv, payload)
	return b64.EncodeToString(payload) + "." + b64.EncodeToString(sig), nil
}

// ErrInvalidToken is returned when a token is malformed or fails verification
// against every trusted key.
var ErrInvalidToken = errors.New("invalid capability token")

// Verify checks the token's signature against any of the trusted public keys
// (a list, to allow key rotation without a flag-day) and returns the decoded
// claims. It does not check expiry — the caller does, so that clock policy
// lives in one place (the KMS). Returns ErrInvalidToken on any failure.
func Verify(tokenStr string, keys []ed25519.PublicKey) (Claims, error) {
	payloadB64, sigB64, ok := strings.Cut(tokenStr, ".")
	if !ok {
		return Claims{}, ErrInvalidToken
	}
	payload, err := b64.DecodeString(payloadB64)
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	sig, err := b64.DecodeString(sigB64)
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	verified := false
	for _, k := range keys {
		if len(k) == ed25519.PublicKeySize && ed25519.Verify(k, payload, sig) {
			verified = true
			break
		}
	}
	if !verified {
		return Claims{}, ErrInvalidToken
	}
	var w wire
	if err := json.Unmarshal(payload, &w); err != nil {
		return Claims{}, ErrInvalidToken
	}
	return w.toClaims(), nil
}

// GenerateKey returns a fresh ed25519 keypair (for tests and key provisioning).
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(nil)
}

// EncodeKey base64-encodes a key (public or private) for env/config transport.
func EncodeKey(key []byte) string { return base64.StdEncoding.EncodeToString(key) }

// ParsePrivateKey decodes a base64 ed25519 private key.
func ParsePrivateKey(b64Std string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64Std))
	if err != nil {
		return nil, fmt.Errorf("decoding private key: %w", err)
	}
	if len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("private key must be %d bytes, got %d", ed25519.PrivateKeySize, len(raw))
	}
	return ed25519.PrivateKey(raw), nil
}

// ParsePublicKeys decodes a comma-separated list of base64 ed25519 public keys
// (the trusted-key set, for rotation).
func ParsePublicKeys(spec string) ([]ed25519.PublicKey, error) {
	var keys []ed25519.PublicKey
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(part)
		if err != nil {
			return nil, fmt.Errorf("decoding public key: %w", err)
		}
		if len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("public key must be %d bytes, got %d", ed25519.PublicKeySize, len(raw))
		}
		keys = append(keys, ed25519.PublicKey(raw))
	}
	return keys, nil
}
