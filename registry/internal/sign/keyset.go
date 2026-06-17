package sign

import (
	"fmt"

	"spade_registry/internal/store"
)

// Keyset manages signing keys persisted in the store: the single active key
// used to sign, and all listed public keys served at /pubkeys. It implements
// the flag-day-free rotation in registry.md §6.1.
type Keyset struct {
	st *store.Store
}

// NewKeyset wraps a store.
func NewKeyset(st *store.Store) *Keyset { return &Keyset{st: st} }

// EnsureActiveKey generates and persists an active key if none exists. Returns
// the active key's id. Used at registryd bootstrap.
func (k *Keyset) EnsureActiveKey() (string, error) {
	active, err := k.st.ActiveSigningKey()
	if err == nil {
		return active.ID, nil
	}
	if err != store.ErrNotFound {
		return "", err
	}
	return k.AddKey()
}

// AddKey generates a new keypair, makes it the sole active key, and lists it.
// Any previously active key is demoted to listed-but-inactive (still served by
// /pubkeys so existing signatures keep verifying until the key is retired).
func (k *Keyset) AddKey() (string, error) {
	kp, err := GenerateKeypair()
	if err != nil {
		return "", err
	}
	if err := k.st.DeactivateSigningKeys(); err != nil {
		return "", err
	}
	row := &store.SigningKey{
		ID:         store.NewID(),
		PublicKey:  kp.Public,
		PrivateKey: kp.Private,
		Active:     true,
		Listed:     true,
	}
	if err := k.st.CreateSigningKey(row); err != nil {
		return "", err
	}
	return row.ID, nil
}

// ImportKey installs a caller-provided keypair as the active key (used when
// SIGNING_KEY_SOURCE=env). Idempotent on the public key.
func (k *Keyset) ImportKey(public, private string) (string, error) {
	if _, err := DecodePublic(public); err != nil {
		return "", err
	}
	if _, err := DecodePrivate(private); err != nil {
		return "", err
	}
	if err := k.st.DeactivateSigningKeys(); err != nil {
		return "", err
	}
	row := &store.SigningKey{
		ID:         store.NewID(),
		PublicKey:  public,
		PrivateKey: private,
		Active:     true,
		Listed:     true,
	}
	if err := k.st.CreateSigningKey(row); err != nil {
		return "", err
	}
	return row.ID, nil
}

// RetireKey unlists and deactivates a key (final rotation step).
func (k *Keyset) RetireKey(id string) error { return k.st.RetireSigningKey(id) }

// SignArtifact signs data with the active key, returning (signature, keyID).
func (k *Keyset) SignArtifact(data []byte) ([]byte, string, error) {
	active, err := k.st.ActiveSigningKey()
	if err == store.ErrNotFound {
		return nil, "", ErrNoActiveKey
	}
	if err != nil {
		return nil, "", err
	}
	sig, err := Sign(active.PrivateKey, data)
	if err != nil {
		return nil, "", fmt.Errorf("signing artifact: %w", err)
	}
	return sig, active.ID, nil
}

// PublicKeys returns the base64 public keys currently served by /pubkeys.
func (k *Keyset) PublicKeys() ([]string, error) {
	rows, err := k.st.ListedSigningKeys()
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(rows))
	for i, r := range rows {
		keys[i] = r.PublicKey
	}
	return keys, nil
}
