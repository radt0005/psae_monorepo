package cmd

import (
	"os"
	"testing"
	"time"
)

func TestRunLogout_ClearsCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)

	if err := SaveCredentials(Credentials{
		Token:     "tok",
		ServerURL: "https://example.com",
		IssuedAt:  time.Now(),
	}); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	if err := runLogout(); err != nil {
		t.Fatalf("runLogout: %v", err)
	}

	if _, err := os.Stat(CredentialsPath()); !os.IsNotExist(err) {
		t.Errorf("expected credentials file to be removed, stat err = %v", err)
	}
}

func TestRunLogout_NoFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)

	if err := runLogout(); err != nil {
		t.Errorf("runLogout with no credentials should not error, got: %v", err)
	}
}
