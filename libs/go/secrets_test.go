package spade

import (
	"os"
	"testing"
)

func resetSecrets() {
	secretsMu.Lock()
	secretsCache = nil
	secretsMu.Unlock()
}

func TestGetSecretReturnsValue(t *testing.T) {
	resetSecrets()
	t.Setenv("SPADE_SECRETS", `{"db":"postgres://user:pw@host/db"}`)
	v, err := GetSecret("db")
	if err != nil {
		t.Fatalf("GetSecret: %v", err)
	}
	if v != "postgres://user:pw@host/db" {
		t.Fatalf("got %q", v)
	}
}

func TestGetSecretMissing(t *testing.T) {
	resetSecrets()
	t.Setenv("SPADE_SECRETS", `{"db":"x"}`)
	if _, err := GetSecret("nope"); err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestGetSecretScrubsEnv(t *testing.T) {
	resetSecrets()
	t.Setenv("SPADE_SECRETS", `{"db":"x"}`)
	if _, err := GetSecret("db"); err != nil {
		t.Fatalf("GetSecret: %v", err)
	}
	if _, ok := os.LookupEnv("SPADE_SECRETS"); ok {
		t.Fatal("SPADE_SECRETS should be scrubbed after load")
	}
}

func TestGetSecretNoEnv(t *testing.T) {
	resetSecrets()
	os.Unsetenv("SPADE_SECRETS")
	if _, err := GetSecret("db"); err == nil {
		t.Fatal("expected error when no secrets provided")
	}
}

func TestParseSecrets(t *testing.T) {
	if len(parseSecrets("")) != 0 {
		t.Fatal("empty blob should parse to empty map")
	}
	m := parseSecrets(`{"a":"1","b":"2"}`)
	if m["a"] != "1" || m["b"] != "2" {
		t.Fatalf("unexpected parse: %v", m)
	}
}
