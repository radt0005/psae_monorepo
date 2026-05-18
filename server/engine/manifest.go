// Package engine wires together the in-memory scheduling primitives in
// core/, the durable store in store/, and the broker transport in
// broker/ to provide the spade-scheduler's coordination layer.
//
// engine.Engine owns:
//   - a *core.MultiTenantScheduler (the ready-set, DAG, map context machinery)
//   - a store.Store (the source of truth per scheduler.md §State Management)
//   - a broker.JobPublisher (dispatches go to spade.jobs)
//   - a ManifestProvider (resolves block name → BlockManifest)
//
// The engine accepts pipeline submissions, drives dispatch when blocks
// become ready, consumes results, persists every transition, and offers
// HTTP-friendly status snapshots.
package engine

import (
	"errors"
	"sync"

	"core"
)

// ManifestProvider resolves a block name (and optional version) to its
// parsed BlockManifest.  In production this is backed by the registry
// metadata mirror in PostgreSQL (registry.md §10); in tests it is an
// in-memory map.
type ManifestProvider interface {
	Lookup(name, version string) (core.BlockManifest, error)
}

// ErrManifestNotFound is returned by ManifestProvider implementations
// when no manifest is registered for the given block name.
var ErrManifestNotFound = errors.New("manifest not found")

// MapManifestProvider is the in-memory ManifestProvider used by tests.
// Construct with NewMapManifestProvider.
type MapManifestProvider struct {
	mu        sync.Mutex
	manifests map[string]core.BlockManifest
}

// NewMapManifestProvider returns a fresh provider.
func NewMapManifestProvider() *MapManifestProvider {
	return &MapManifestProvider{manifests: map[string]core.BlockManifest{}}
}

// Set registers a manifest for the given block name.  Subsequent
// Lookup calls return this manifest regardless of the version arg.
func (m *MapManifestProvider) Set(name string, manifest core.BlockManifest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.manifests[name] = manifest
}

// Lookup returns the registered manifest or ErrManifestNotFound.
func (m *MapManifestProvider) Lookup(name, version string) (core.BlockManifest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	manifest, ok := m.manifests[name]
	if !ok {
		return core.BlockManifest{}, ErrManifestNotFound
	}
	return manifest, nil
}

// All returns a copy of every registered (name → manifest) entry.  Used
// by tests and by engine code that needs the full map for pipeline
// validation.
func (m *MapManifestProvider) All() map[string]core.BlockManifest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]core.BlockManifest, len(m.manifests))
	for k, v := range m.manifests {
		out[k] = v
	}
	return out
}
