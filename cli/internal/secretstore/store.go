// Package secretstore stores local Spade secrets in the operating-system
// keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)
// via github.com/zalando/go-keyring. Local secrets are used by `spade run`;
// cloud secrets live separately in the KMS (see spec/secrets.md §8).
//
// go-keyring cannot enumerate entries, so this package maintains a small index
// of secret names (names are not sensitive) under a reserved keychain key to
// back the List operation.
package secretstore

import (
	"encoding/json"
	"errors"
	"sort"

	"github.com/zalando/go-keyring"
)

// service is the keychain service under which all Spade secrets are stored.
const service = "spade"

// indexKey is a reserved keychain entry holding the JSON list of secret names.
const indexKey = "__spade_secret_index__"

// ErrNotFound is returned by Get when a secret is not in the keychain.
var ErrNotFound = keyring.ErrNotFound

// Set stores a secret value under name and records the name in the index.
func Set(name, value string) error {
	if err := keyring.Set(service, name, value); err != nil {
		return err
	}
	return addToIndex(name)
}

// Get returns the value stored under name, or ErrNotFound if absent.
func Get(name string) (string, error) {
	return keyring.Get(service, name)
}

// Delete removes a secret and its index entry. Deleting a missing secret is
// not an error.
func Delete(name string) error {
	if err := keyring.Delete(service, name); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	return removeFromIndex(name)
}

// List returns the names of stored secrets, sorted.
func List() ([]string, error) {
	return readIndex()
}

func readIndex() ([]string, error) {
	raw, err := keyring.Get(service, indexKey)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	if err := json.Unmarshal([]byte(raw), &names); err != nil {
		return nil, err
	}
	return names, nil
}

func writeIndex(names []string) error {
	sort.Strings(names)
	raw, err := json.Marshal(names)
	if err != nil {
		return err
	}
	return keyring.Set(service, indexKey, string(raw))
}

func addToIndex(name string) error {
	names, err := readIndex()
	if err != nil {
		return err
	}
	for _, n := range names {
		if n == name {
			return nil
		}
	}
	return writeIndex(append(names, name))
}

func removeFromIndex(name string) error {
	names, err := readIndex()
	if err != nil {
		return err
	}
	kept := make([]string, 0, len(names))
	for _, n := range names {
		if n != name {
			kept = append(kept, n)
		}
	}
	return writeIndex(kept)
}
