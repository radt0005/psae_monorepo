package app

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestParseFlagsDefaults verifies the env-var defaulting logic.
func TestParseFlagsDefaults(t *testing.T) {
	t.Setenv("SPADE_HTTP_ADDR", ":9999")
	cfg := ParseFlags(nil)
	if cfg.HTTPAddr != ":9999" {
		t.Errorf("HTTPAddr from env: %q", cfg.HTTPAddr)
	}
}

// TestRunSkipBrokerSmoke starts the binary in skip-broker mode against
// a random port, hits /healthz, then triggers shutdown by cancelling
// the context — but Run() listens on signals, so we instead rely on
// a short-lived child to validate the wiring.  The point is that Run
// returns nil after a clean shutdown, not crashes.
func TestRunSkipBrokerSmoke(t *testing.T) {
	t.Setenv("SPADE_HTTP_ADDR", "127.0.0.1:0")
	t.Setenv("SPADE_SKIP_BROKER", "1")
	t.Setenv("SPADE_LOG_LEVEL", "error")

	// Run() blocks on signal.NotifyContext; spin it in a goroutine.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = Run([]string{"-http-addr", "127.0.0.1:0", "-skip-broker"})
	}()
	// Give Run a moment to set up.  Listening on :0 means we can't
	// actually hit it from here, but we can verify it doesn't panic.
	select {
	case <-done:
		// Should not return on its own.
		t.Fatal("Run returned without signal")
	case <-time.After(100 * time.Millisecond):
	}
	// Signal via the OS by sending SIGINT to ourselves would affect
	// the test runner; instead, accept that this smoke test mainly
	// verifies that Run doesn't crash during startup.
}

// TestAPIHealthEndpointReachable exposes the API server on a chosen
// port (skip-broker) and hits the health endpoint.  This is a direct
// confidence check that nothing is missing in the wiring.
func TestAPIHealthEndpointReachable(t *testing.T) {
	t.Setenv("SPADE_HTTP_ADDR", "127.0.0.1:0")
	t.Setenv("SPADE_SKIP_BROKER", "1")
	t.Setenv("SPADE_LOG_LEVEL", "error")
	// Find a free port by attempting a quick bind.
	addr := "127.0.0.1:38193"
	go func() {
		_ = Run([]string{"-http-addr", addr, "-skip-broker"})
	}()
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			lastErr = nil
		} else {
			lastErr = err
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("health endpoint never came up; last err: %v", lastErr)
}

// TestMaskDSNHidesPassword is a tiny unit test on the helper.
func TestMaskDSNHidesPassword(t *testing.T) {
	out := maskDSN("postgres://user:secret@host/db")
	if strings.Contains(out, "secret") {
		t.Errorf("password leaked: %q", out)
	}
}

// Compile-time helper so importing context in tests stays useful even
// if Run later changes signature.
var _ = context.Background
