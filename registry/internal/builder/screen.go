// Package builder is the build worker that runs inside a per-language container.
// It clones the source at the published commit, screens it, builds the
// language-specific artifact, uploads the unsigned tarball to S3 staging, and
// reports the result back to the registry over its HTTP API. It holds no
// database access and never holds the signing key (the control plane signs).
package builder

import "context"

// ScreenResult is the outcome of a screener run.
type ScreenResult struct {
	ScreenerName    string
	ScreenerVersion string
	Passed          bool
	Details         string
}

// Screener inspects cloned source before the build runs. Screening before build
// is the foundation of the trust chain (registry.md §1): the registry, not the
// developer, controls what gets built.
type Screener interface {
	Screen(ctx context.Context, srcDir string) (ScreenResult, error)
}

// NoopScreener passes everything. It is the placeholder for the future AI
// screening agent (registry.md §4: screening is a pluggable policy hook).
type NoopScreener struct{}

// Screen always passes.
func (NoopScreener) Screen(ctx context.Context, srcDir string) (ScreenResult, error) {
	return ScreenResult{
		ScreenerName:    "noop",
		ScreenerVersion: "0",
		Passed:          true,
		Details:         "no screening configured",
	}, nil
}
