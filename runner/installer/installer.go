package installer

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"core"
)

// Default artifact coordinates for the worker fleet. A single interpreter version
// rides along with the base image (notes.md A3), so platform/arch fully key the
// artifact; a multi-interpreter ABI tag is deferred (plan Phase 9).
const (
	DefaultPlatform = "linux"
	DefaultArch     = "amd64"
)

// PubKeySource yields the trusted public key set for signature verification.
// Phase 7's cache implements this; tests supply a static set.
type PubKeySource interface {
	Keys(ctx context.Context) ([]string, error)
}

// Rejected marks a permanent, security- or policy-based refusal to install: a bad
// signature, a content-hash mismatch, or a non-servable version state
// (recalled/yanked/not-found). The worker maps a Rejected to a *block* failure
// plus a poison marker; every other error (network, 5xx) is transient/infra and
// maps to a nack+redeliver, so a registry blip does not mass-fail pipelines.
type Rejected struct {
	Reason string
	Err    error
}

func (r *Rejected) Error() string {
	if r.Err != nil {
		return fmt.Sprintf("artifact rejected (%s): %v", r.Reason, r.Err)
	}
	return "artifact rejected: " + r.Reason
}

func (r *Rejected) Unwrap() error { return r.Err }

// IsRejected reports whether err is (or wraps) a permanent Rejected refusal.
func IsRejected(err error) bool {
	var r *Rejected
	return errors.As(err, &r)
}

// Installer fetches, verifies, unpacks, and indexes registry artifacts.
type Installer struct {
	Client    *Client
	PubKeys   PubKeySource
	Registry  *core.BlockRegistry
	BlocksDir string // ~/.spade/blocks
	Platform  string // defaults to DefaultPlatform
	Arch      string // defaults to DefaultArch

	locks sync.Map // per "<collection>/<version>" *sync.Mutex, so concurrent jobs fetch once
}

// New builds an Installer with default platform/arch.
func New(client *Client, keys PubKeySource, registry *core.BlockRegistry, blocksDir string) *Installer {
	return &Installer{
		Client:    client,
		PubKeys:   keys,
		Registry:  registry,
		BlocksDir: blocksDir,
		Platform:  DefaultPlatform,
		Arch:      DefaultArch,
	}
}

// Install ensures collection@version is unpacked and indexed locally, fetching and
// verifying it from the registry on a miss. It is idempotent and safe for
// concurrent calls for the same collection/version: the first installs, the rest
// observe the completed install and return without re-fetching.
func (in *Installer) Install(ctx context.Context, collection, version string) error {
	key := collection + "/" + version
	mu, _ := in.locks.LoadOrStore(key, &sync.Mutex{})
	lock := mu.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	installed, err := in.alreadyInstalled(collection, version)
	if err != nil {
		return err
	}
	if installed {
		return nil
	}
	return in.fetchVerifyUnpack(ctx, collection, version)
}

// alreadyInstalled reports whether the local index already has this collection
// version. The index (not the on-disk dir) is the completion marker: registration
// is the last step, so an index hit means a prior install finished cleanly.
func (in *Installer) alreadyInstalled(collection, version string) (bool, error) {
	blocks, err := in.Registry.ListBlocks()
	if err != nil {
		return false, err
	}
	for _, b := range blocks {
		if b.CollectionName == collection && b.CollectionVersion == version {
			return true, nil
		}
	}
	return false, nil
}

func (in *Installer) fetchVerifyUnpack(ctx context.Context, collection, version string) error {
	platform, arch := in.Platform, in.Arch

	// 1. Metadata: content hash to verify against + current state. A non-servable
	// state is a permanent rejection (feeds recall handling); 404/410 likewise.
	meta, err := in.Client.fetchMeta(ctx, collection, version, platform, arch)
	if err != nil {
		return classify(err, "fetching artifact metadata")
	}
	switch meta.State {
	case "available", "deprecated":
		// installable
	default:
		return &Rejected{Reason: "version state " + meta.State}
	}

	// 2. Download the tarball and its detached signature.
	tarball, err := in.Client.fetchTarball(ctx, collection, version, platform, arch)
	if err != nil {
		return classify(err, "downloading artifact")
	}
	sig, err := in.Client.fetchSig(ctx, collection, version, platform, arch)
	if err != nil {
		return classify(err, "downloading signature")
	}

	// 3. Verify signature (against the trusted key list) then content hash. Either
	// failure is a hard rejection: forging an artifact needs the registry key.
	keys, err := in.PubKeys.Keys(ctx)
	if err != nil {
		return fmt.Errorf("loading trusted keys: %w", err) // transient: keys unavailable
	}
	if !core.VerifySignature(keys, tarball, sig) {
		return &Rejected{Reason: "signature verification failed"}
	}
	if !core.HashMatches(tarball, meta.ContentHash) {
		return &Rejected{Reason: "content hash mismatch"}
	}

	// 4. Unpack into a temp dir on the same filesystem, then atomically rename into
	// place so a crash never leaves a half-written install for the index to trust.
	destDir := filepath.Join(in.BlocksDir, collection, version)
	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(in.BlocksDir, ".tmp-install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp) // no-op after a successful rename
	if err := core.Unpack(bytes.NewReader(tarball), tmp); err != nil {
		return &Rejected{Reason: "unpacking artifact", Err: err}
	}
	if err := os.RemoveAll(destDir); err != nil {
		return err
	}
	if err := os.Rename(tmp, destDir); err != nil {
		return err
	}

	// 5. Register one index entry per block manifest, recording registry
	// provenance so the recall-freshness path engages for these entries.
	return in.register(destDir, collection, version, meta.ContentHash, sig)
}

func (in *Installer) register(destDir, collection, version, contentHash string, sig []byte) error {
	lang, err := core.DetectLanguage(destDir)
	if err != nil {
		return fmt.Errorf("detecting language of %s: %w", collection, err)
	}
	manifestPaths, err := core.DiscoverBlocks(destDir)
	if err != nil {
		return fmt.Errorf("discovering blocks: %w", err)
	}
	sigB64 := base64.StdEncoding.EncodeToString(sig)
	now := time.Now().UTC()
	for _, p := range manifestPaths {
		m, err := core.LoadBlockManifest(p)
		if err != nil {
			return fmt.Errorf("loading manifest %s: %w", p, err)
		}
		shortName := m.ID
		if i := lastDot(shortName); i >= 0 {
			shortName = shortName[i+1:]
		}
		entry := core.BlockRegistryEntry{
			CollectionName:    collection,
			CollectionVersion: version,
			BlockName:         shortName,
			BlockID:           m.ID,
			Language:          string(lang),
			Entrypoint:        m.Entrypoint,
			InstalledPath:     destDir,
			ContentHash:       contentHash,
			Kind:              string(m.Kind),
			Network:           m.Network,
			Source:            core.InstallSourceRegistry,
			Signature:         sigB64,
			RegistryState:     "available",
			LastVerifiedAt:    now,
		}
		if err := in.Registry.RegisterBlock(entry); err != nil {
			return fmt.Errorf("registering block %s: %w", m.ID, err)
		}
	}
	return nil
}

// Recheck confirms a locally-installed collection version is still servable,
// driving the recall-freshness path (worker.md §Recall). It returns:
//   - nil and bumps the index freshness when the version is available/deprecated;
//   - a *Rejected when the version is recalled/yanked/gone, after evicting the
//     local install and index entries so the block cannot run;
//   - a plain (transient) error when the registry is unreachable, leaving the
//     install in place so the worker can proceed best-effort.
func (in *Installer) Recheck(ctx context.Context, collection, version string) error {
	meta, err := in.Client.fetchMeta(ctx, collection, version, in.Platform, in.Arch)
	if err != nil {
		if c := classify(err, "rechecking version"); IsRejected(c) {
			// 404/410: the version is gone — treat as a recall and evict.
			_ = in.evict(collection, version)
			return c
		}
		return err // transient: keep the install, proceed best-effort
	}
	switch meta.State {
	case "available", "deprecated":
		return in.Registry.TouchCollection(collection, version, meta.State, time.Now().UTC())
	default:
		_ = in.evict(collection, version)
		return &Rejected{Reason: "version " + meta.State}
	}
}

// evict removes a collection version from the index and disk.
func (in *Installer) evict(collection, version string) error {
	derr := in.Registry.DeleteCollection(collection, version)
	rerr := os.RemoveAll(filepath.Join(in.BlocksDir, collection, version))
	if derr != nil {
		return derr
	}
	return rerr
}

func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

// classify turns a client error into either a permanent Rejected (404/410 — the
// artifact is gone or non-servable) or a transient error (5xx, transport) that
// the worker should retry via redelivery.
func classify(err error, ctx string) error {
	var he *httpError
	if errors.As(err, &he) {
		if he.Status == 404 || he.Status == 410 {
			return &Rejected{Reason: fmt.Sprintf("%s: %s", ctx, he.Error())}
		}
	}
	return fmt.Errorf("%s: %w", ctx, err)
}
