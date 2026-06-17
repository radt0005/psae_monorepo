package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"core"

	"spade_registry/internal/blob"
	"spade_registry/internal/wire"
)

// Deps are the injectable collaborators for Run, so the orchestration can be
// tested without a container, a real registry, or S3.
type Deps struct {
	Client   *Client
	Cloner   Cloner
	Screener Screener
	Blob     blob.Store
	// BuilderFor selects a Builder for a language; defaults to BuilderFor.
	BuilderFor func(lang string) (Builder, error)
}

// Run executes the full build-worker flow for one job:
//
//	job → clone@sha → screen → (report) → build → package → upload staging → complete
//
// On any failure after the job is fetched, it reports failure to the registry.
// It returns an error for observability, but the authoritative state lives in
// the registry (set via the complete/fail callbacks).
func Run(ctx context.Context, d Deps) error {
	if d.BuilderFor == nil {
		d.BuilderFor = BuilderFor
	}
	job, err := d.Client.Job(ctx)
	if err != nil {
		return fmt.Errorf("fetching build job: %w", err)
	}

	if err := runBuild(ctx, d, job); err != nil {
		_ = d.Client.Fail(ctx, wire.FailRequest{Reason: err.Error()})
		return err
	}
	return nil
}

func runBuild(ctx context.Context, d Deps, job wire.BuildJobDetail) error {
	// 1. Clone the exact commit (the registry controls the bytes — §1).
	srcDir, cleanup, err := d.Cloner.Clone(ctx, job.RepoURL, job.CommitSHA)
	if err != nil {
		return fmt.Errorf("clone: %w", err)
	}
	defer cleanup()

	// 2. Screen before building.
	res, err := d.Screener.Screen(ctx, srcDir)
	if err != nil {
		return fmt.Errorf("screening: %w", err)
	}
	proceed, err := d.Client.ReportScreening(ctx, wire.ScreeningReport{
		ScreenerName:    res.ScreenerName,
		ScreenerVersion: res.ScreenerVersion,
		Passed:          res.Passed,
		Details:         res.Details,
	})
	if err != nil {
		return fmt.Errorf("reporting screening: %w", err)
	}
	if !res.Passed {
		return fmt.Errorf("screening failed: %s", res.Details)
	}
	if !proceed {
		// Approval required before building; the registry holds the version at
		// `screened`. Nothing more for this ephemeral build container to do.
		return nil
	}

	// 3. Build with the language-specific builder.
	b, err := d.BuilderFor(job.Language)
	if err != nil {
		return err
	}
	artifactDir, err := b.Build(ctx, srcDir, job.Collection, job.Version)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}
	defer os.RemoveAll(artifactDir)

	// 4. Package to a temp tarball and compute its content hash.
	tarPath := filepath.Join(os.TempDir(), "artifact-"+job.JobID+".tar.gz")
	f, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	hash, err := PackageTarGz(artifactDir, f)
	f.Close()
	if err != nil {
		return fmt.Errorf("packaging: %w", err)
	}
	defer os.Remove(tarPath)

	// 5. Upload the unsigned tarball to the staging prefix. The control plane
	//    verifies the hash and signs (the builder never holds the key).
	rf, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	info, _ := rf.Stat()
	stagingKey := job.StagingKey
	if stagingKey == "" {
		stagingKey = job.StagingPrefix + job.Collection + "/" + job.Version + "/" +
			TargetPlatform + "/" + TargetArch + ".tar.gz"
	}
	err = d.Blob.Put(ctx, stagingKey, rf, info.Size(), "application/gzip")
	rf.Close()
	if err != nil {
		return fmt.Errorf("uploading staging artifact: %w", err)
	}

	// 6. Collect block metadata and report completion.
	blocks, err := collectBlocks(srcDir)
	if err != nil {
		return fmt.Errorf("collecting block metadata: %w", err)
	}
	return d.Client.Complete(ctx, wire.CompleteRequest{
		Platform:    TargetPlatform,
		Arch:        TargetArch,
		ContentHash: hash,
		StagingKey:  stagingKey,
		SizeBytes:   info.Size(),
		Blocks:      blocks,
	})
}

// collectBlocks scans blocks/*.yaml in srcDir and converts them to wire form.
func collectBlocks(srcDir string) ([]wire.BlockManifestWire, error) {
	paths, err := core.DiscoverBlocks(srcDir)
	if err != nil {
		return nil, err
	}
	var out []wire.BlockManifestWire
	for _, p := range paths {
		m, err := core.LoadBlockManifest(p)
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(filepath.Base(p), ".yaml")
		out = append(out, convertManifest(name, m))
	}
	return out, nil
}

func convertManifest(name string, m core.BlockManifest) wire.BlockManifestWire {
	return wire.BlockManifestWire{
		ID:          m.ID,
		Name:        name,
		Version:     m.Version,
		Kind:        string(m.Kind),
		Network:     m.Network,
		Description: m.Description,
		Entrypoint:  m.Entrypoint,
		Inputs:      toAnyMap(m.Inputs),
		Outputs:     toAnyMap(m.Outputs),
	}
}

// toAnyMap converts a typed declaration map to a generic JSON map.
func toAnyMap(v any) map[string]any {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}
