package main

import (
	"path/filepath"
	"strings"
	"testing"

	"core"
)

// TestGetenvFallsBack verifies the env-var helper returns the default
// when the variable is unset or empty.
func TestGetenvFallsBack(t *testing.T) {
	t.Setenv("SPADE_TEST_VAR", "")
	got := getenv("SPADE_TEST_VAR", "fallback")
	if got != "fallback" {
		t.Errorf("expected fallback, got %q", got)
	}

	t.Setenv("SPADE_TEST_VAR", "set")
	got = getenv("SPADE_TEST_VAR", "fallback")
	if got != "set" {
		t.Errorf("expected 'set', got %q", got)
	}
}

func TestNewLoggerLevels(t *testing.T) {
	for _, lvl := range []string{"debug", "info", "warn", "error", "unknown"} {
		if got := newLogger(lvl); got == nil {
			t.Errorf("newLogger(%q) returned nil", lvl)
		}
	}
}

func TestWorkerOptionsRegistryToggle(t *testing.T) {
	reg, err := core.OpenRegistry(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	logger := newLogger("error")

	// No registry URL → legacy path: no installer, no pubkey cache.
	_, pubkeys := workerOptions(config{}, reg, logger)
	if pubkeys != nil {
		t.Error("without RegistryURL the pubkey cache (and installer) must be absent")
	}

	// Registry URL set → fetch installer is constructed with a pubkey cache.
	_, pubkeys = workerOptions(config{RegistryURL: "https://registry.example", WorkerToken: "t", FreshnessSec: 3600}, reg, logger)
	if pubkeys == nil {
		t.Error("with RegistryURL the installer + pubkey cache must be constructed")
	}
}

func TestEnvIntFallback(t *testing.T) {
	if got := envInt("SPADE_MISSING_INT", 42); got != 42 {
		t.Errorf("unset env should fall back to 42, got %d", got)
	}
	t.Setenv("SPADE_SOME_INT", "notanint")
	if got := envInt("SPADE_SOME_INT", 7); got != 7 {
		t.Errorf("unparseable env should fall back to 7, got %d", got)
	}
	t.Setenv("SPADE_SOME_INT", "15")
	if got := envInt("SPADE_SOME_INT", 7); got != 15 {
		t.Errorf("valid env should parse to 15, got %d", got)
	}
}

func TestCheckIsolateProducesHelpfulError(t *testing.T) {
	// Most CI environments lack isolate; this test verifies the error
	// message points at the spec.
	err := checkIsolate()
	if err == nil {
		t.Skip("isolate is installed on this host; skipping negative case")
	}
	if !strings.Contains(err.Error(), "isolate") {
		t.Errorf("error should mention isolate: %v", err)
	}
}
