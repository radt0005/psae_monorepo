package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Credentials holds the stored login session.
type Credentials struct {
	Token     string    `json:"token"`
	ServerURL string    `json:"server_url"`
	IssuedAt  time.Time `json:"issued_at"`
}

// SaveCredentials writes credentials to ~/.spade/auth/credentials.json
// with mode 0600 so only the owning user can read it.
func SaveCredentials(creds Credentials) error {
	if err := os.MkdirAll(AuthDir(), 0700); err != nil {
		return fmt.Errorf("creating auth directory: %w", err)
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(CredentialsPath(), data, 0600)
}

// LoadCredentials reads credentials from disk.
// Returns a descriptive error if no credentials are stored.
func LoadCredentials() (Credentials, error) {
	data, err := os.ReadFile(CredentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return Credentials{}, fmt.Errorf("not logged in: run 'spade login'")
		}
		return Credentials{}, fmt.Errorf("reading credentials: %w", err)
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return Credentials{}, fmt.Errorf("parsing credentials: %w", err)
	}
	return creds, nil
}

// ClearCredentials removes the stored credentials file.
// Returns nil if the file does not exist.
func ClearCredentials() error {
	err := os.Remove(CredentialsPath())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
