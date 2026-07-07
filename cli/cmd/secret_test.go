package cmd

import (
	"core"
	"strings"
	"testing"

	"spade/internal/secretstore"

	"github.com/zalando/go-keyring"
)

func TestResolveBlockSecrets(t *testing.T) {
	keyring.MockInit()
	if err := secretstore.Set("prod-dsn", "postgres://prod"); err != nil {
		t.Fatalf("seeding keychain: %v", err)
	}

	pb := core.PipelineBlock{Secrets: map[string]string{"db": "prod-dsn"}}
	got, err := resolveBlockSecrets(pb)
	if err != nil {
		t.Fatalf("resolveBlockSecrets: %v", err)
	}
	if got["db"] != "postgres://prod" {
		t.Fatalf("got %v, want db=postgres://prod (logical name re-keyed)", got)
	}
}

func TestResolveBlockSecretsNoneDeclared(t *testing.T) {
	keyring.MockInit()
	got, err := resolveBlockSecrets(core.PipelineBlock{})
	if err != nil {
		t.Fatalf("resolveBlockSecrets: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for a block with no secrets, got %v", got)
	}
}

func TestResolveBlockSecretsMissing(t *testing.T) {
	keyring.MockInit()
	pb := core.PipelineBlock{Secrets: map[string]string{"db": "absent"}}
	_, err := resolveBlockSecrets(pb)
	if err == nil {
		t.Fatal("expected an error for a secret missing from the keychain")
	}
	if !strings.Contains(err.Error(), "spade secret set absent") {
		t.Fatalf("error should guide the user to set the secret: %v", err)
	}
}
