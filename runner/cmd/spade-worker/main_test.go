package main

import (
	"strings"
	"testing"
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
