// Package app holds the spade-scheduler binary entrypoint.  The thin
// top-level main.go wrapper just calls Main here.  Pulling the body out
// of package main lets tests exercise the wiring without needing to
// fork-exec the binary.
package app

// Main is the entrypoint invoked by ../../main.go.  Real implementation
// is wired up in Phase 8 of IMPLEMENTATION_PLAN.md; this stub exists so
// the rest of the packages can compile while the engine, store, and API
// layers are being built.
func Main() {
	runMain()
}
