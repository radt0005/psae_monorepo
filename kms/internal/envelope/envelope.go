// Package envelope implements envelope encryption for secret values
// (spec/secrets.md §5.1): each value is encrypted with a per-secret data
// encryption key (DEK), and the DEK is wrapped by a key-encryption key (KEK)
// held only by the KMS process. A database compromise alone yields only
// ciphertext; decryption requires a KEK.
package envelope

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// keySize is the AES-256 key length in bytes for both KEKs and DEKs.
const keySize = 32

// KeySet holds the key-encryption keys by id and the active id used to wrap new
// secrets. Accepting a set (not a single key) supports rotation without a
// flag-day: old keys stay available to unwrap existing secrets while new ones
// are wrapped under the active key (spec/secrets.md §9).
type KeySet struct {
	keys     map[string][]byte
	activeID string
}

// NewKeySet validates the keys (each must be 32 bytes) and the active id.
func NewKeySet(keys map[string][]byte, activeID string) (*KeySet, error) {
	if len(keys) == 0 {
		return nil, errors.New("no KEKs provided")
	}
	for id, k := range keys {
		if len(k) != keySize {
			return nil, fmt.Errorf("KEK %q must be %d bytes, got %d", id, keySize, len(k))
		}
	}
	if _, ok := keys[activeID]; !ok {
		return nil, fmt.Errorf("active KEK id %q not present in the key set", activeID)
	}
	return &KeySet{keys: keys, activeID: activeID}, nil
}

// Sealed is the stored ciphertext form of a secret value.
type Sealed struct {
	Ciphertext []byte // value encrypted under the DEK
	ValueNonce []byte // GCM nonce for the value
	WrappedDEK []byte // DEK encrypted under the KEK
	DEKNonce   []byte // GCM nonce for the wrapped DEK
	KEKID      string // which KEK wrapped the DEK
}

// Seal encrypts plaintext under a fresh DEK and wraps the DEK under the active
// KEK.
func (ks *KeySet) Seal(plaintext []byte) (Sealed, error) {
	dek := make([]byte, keySize)
	if _, err := rand.Read(dek); err != nil {
		return Sealed{}, fmt.Errorf("generating DEK: %w", err)
	}
	ct, valNonce, err := gcmSeal(dek, plaintext)
	if err != nil {
		return Sealed{}, fmt.Errorf("encrypting value: %w", err)
	}
	wrapped, dekNonce, err := gcmSeal(ks.keys[ks.activeID], dek)
	if err != nil {
		return Sealed{}, fmt.Errorf("wrapping DEK: %w", err)
	}
	return Sealed{
		Ciphertext: ct,
		ValueNonce: valNonce,
		WrappedDEK: wrapped,
		DEKNonce:   dekNonce,
		KEKID:      ks.activeID,
	}, nil
}

// Open unwraps the DEK with the recorded KEK and decrypts the value.
func (ks *KeySet) Open(s Sealed) ([]byte, error) {
	kek, ok := ks.keys[s.KEKID]
	if !ok {
		return nil, fmt.Errorf("unknown KEK id %q", s.KEKID)
	}
	dek, err := gcmOpen(kek, s.WrappedDEK, s.DEKNonce)
	if err != nil {
		return nil, fmt.Errorf("unwrapping DEK: %w", err)
	}
	pt, err := gcmOpen(dek, s.Ciphertext, s.ValueNonce)
	if err != nil {
		return nil, fmt.Errorf("decrypting value: %w", err)
	}
	return pt, nil
}

func gcmSeal(key, plaintext []byte) (ciphertext, nonce []byte, err error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nonce, nil
}

func gcmOpen(key, ciphertext, nonce []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// ParseKeys parses a "id:base64,id2:base64" specification (as delivered via the
// KMS environment) into a KEK map.
func ParseKeys(spec string) (map[string][]byte, error) {
	keys := map[string][]byte{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, b64, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("malformed KEK entry %q (want id:base64)", part)
		}
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
		if err != nil {
			return nil, fmt.Errorf("decoding KEK %q: %w", id, err)
		}
		keys[strings.TrimSpace(id)] = key
	}
	return keys, nil
}
