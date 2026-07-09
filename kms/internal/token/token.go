// Package token defines the capability token contract between the scheduler
// (which mints tokens) and the KMS (which verifies them) — spec/secrets.md §6.
//
// The ed25519 verifier is added in Phase 5 (SECRETS_IMPLEMENTATION_PLAN §6).
// This package defines the Claims, the Verifier seam the /resolve handler
// depends on, and an Unconfigured placeholder used until the real verifier is
// wired.
package token

import (
	"errors"
	"time"
)

// Claims is the capability the scheduler grants for one invocation: which
// secrets, for which owner, until when.
type Claims struct {
	UserID       string
	InvocationID string
	SecretNames  []string
	Expiry       time.Time
}

// Allows reports whether the token scopes access to the given secret name.
func (c Claims) Allows(name string) bool {
	for _, n := range c.SecretNames {
		if n == name {
			return true
		}
	}
	return false
}

// Expired reports whether the token is past its expiry.
func (c Claims) Expired(now time.Time) bool {
	return !c.Expiry.IsZero() && now.After(c.Expiry)
}

// Verifier turns a raw capability token into verified Claims.
type Verifier interface {
	Verify(raw string) (Claims, error)
}

// ErrUnconfigured is returned by the placeholder verifier until the ed25519
// verifier is wired in Phase 5.
var ErrUnconfigured = errors.New("capability token verification is not configured")

// Unconfigured rejects all tokens. It is the default until Phase 5 supplies the
// scheduler's public key and an ed25519 verifier.
type Unconfigured struct{}

// Verify always fails with ErrUnconfigured.
func (Unconfigured) Verify(string) (Claims, error) { return Claims{}, ErrUnconfigured }
