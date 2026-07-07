package token

import (
	"crypto/ed25519"

	"captoken"
)

// Ed25519Verifier verifies capability tokens against the scheduler's trusted
// public keys (a list, for rotation — spec/secrets.md §6.1, §9). It is the
// production Verifier, replacing Unconfigured once the keys are configured.
type Ed25519Verifier struct {
	keys []ed25519.PublicKey
}

// NewEd25519Verifier builds a verifier over the trusted public keys.
func NewEd25519Verifier(keys []ed25519.PublicKey) *Ed25519Verifier {
	return &Ed25519Verifier{keys: keys}
}

// Verify checks the token signature and returns its claims. Signature failures
// map to a token error; expiry is enforced separately by the handler.
func (v *Ed25519Verifier) Verify(raw string) (Claims, error) {
	c, err := captoken.Verify(raw, v.keys)
	if err != nil {
		return Claims{}, err
	}
	return Claims{
		UserID:       c.UserID,
		InvocationID: c.InvocationID,
		SecretNames:  c.SecretNames,
		Expiry:       c.Expiry,
	}, nil
}
