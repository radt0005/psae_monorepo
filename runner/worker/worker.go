// Package worker provides single-job orchestration for the Spade runner.
//
// A Worker takes a single Job (a core.WorkerAssignment plus the pipeline and
// manifest context the scheduler attached), drives it through core.Execute,
// and returns a core.WorkerResult ready to publish on the spade.results
// queue.
//
// Worker.Run's failure-mode convention is deliberate:
//
//   - Block-level failures (missing block, hash mismatch, non-zero exit,
//     malformed expansion) are returned as WorkerResult with
//     Status == ExecutionStatusError and a nil error.  The caller should
//     publish the result and ack the job.
//
//   - Infrastructure failures (closed registry handle, unreadable work
//     root) are returned as a non-nil error with a zero-value result.
//     The caller should NOT publish a result and should nack the job so
//     the broker can redeliver to another worker.
//
// This matches the two-mode semantics in ../../spec/worker.md §Error Handling.
package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"core"
	spade "spade_runner"
	"spade_runner/installer"
)

// Option configures a Worker at construction time.
type Option func(*Worker)

// WithCache enables output caching at the given cache directory.
// When enabled, successful block outputs are stored keyed by
// core.ComputeCacheKey and subsequent runs with the same key are
// restored from cache without re-executing the block.
func WithCache(cacheDir string) Option {
	return func(w *Worker) {
		w.cacheDir = cacheDir
		w.useCache = cacheDir != ""
	}
}

// Executor abstracts core.Execute so tests can inject a fake without
// needing the isolate sandbox or a built block binary.
type Executor interface {
	Execute(
		block core.BlockInvocation,
		pipelineDir string,
		manifest core.BlockManifest,
		entry core.BlockRegistryEntry,
		reg *core.BlockRegistry,
		secrets map[string]string,
	) (core.BlockInvocationResult, error)
}

// coreExecutor is the production Executor — delegates directly to core.Execute.
type coreExecutor struct{}

func (coreExecutor) Execute(
	block core.BlockInvocation,
	pipelineDir string,
	manifest core.BlockManifest,
	entry core.BlockRegistryEntry,
	reg *core.BlockRegistry,
	secrets map[string]string,
) (core.BlockInvocationResult, error) {
	return core.Execute(block, pipelineDir, manifest, entry, reg, secrets)
}

// WithExecutor overrides the default core.Execute-backed executor.
// Primarily intended for testing.
func WithExecutor(e Executor) Option {
	return func(w *Worker) {
		w.executor = e
	}
}

// Installer fetches, verifies, and installs a collection artifact from the
// registry on a lookup miss, and re-checks an installed version's registry state
// for recalls (see spade_runner/installer). A nil installer keeps the legacy
// behavior: a miss is a terminal block failure with no fetch and no re-check.
type Installer interface {
	Install(ctx context.Context, collection, version string) error
	Recheck(ctx context.Context, collection, version string) error
}

// WithInstaller enables the registry-fetch install path. When set, a lookup miss
// for a block whose assignment pins a CollectionVersion triggers an install +
// re-lookup instead of failing immediately.
func WithInstaller(in Installer) Option {
	return func(w *Worker) {
		w.installer = in
	}
}

// WithFreshness sets how long a registry-installed block's last verification is
// trusted before the worker re-checks its state (for recalls) on the next run.
// Zero (the default) disables re-checking.
func WithFreshness(d time.Duration) Option {
	return func(w *Worker) {
		w.freshness = d
	}
}

// SecretResolver exchanges a job's capability token for the values of the
// stored secrets an invocation's block declared (spec/secrets.md §6). It is
// satisfied by spade_runner/kmsclient in production and a fake in tests.
type SecretResolver interface {
	Resolve(ctx context.Context, token string, names []string) (map[string]string, error)
}

// WithSecretResolver enables secret resolution. When set, an invocation whose
// block declares secrets has them fetched from the KMS and injected into the
// sandbox. A nil resolver (the default) means a block that declares secrets
// fails as a worker-side error.
func WithSecretResolver(r SecretResolver) Option {
	return func(w *Worker) {
		w.secretResolver = r
	}
}

// Worker orchestrates a single block invocation end-to-end.
//
// A Worker is safe to reuse across many jobs; it holds only configuration
// and does not carry per-job state.
type Worker struct {
	Registry *core.BlockRegistry
	WorkRoot string

	cacheDir       string
	useCache       bool
	executor       Executor
	installer      Installer
	freshness      time.Duration
	secretResolver SecretResolver

	// poisoned records <collection>/<version> pairs whose install was
	// permanently rejected (bad signature, hash mismatch, recalled). The worker
	// refuses to re-hit the registry for a poisoned pair until it restarts / an
	// operator investigates (worker.md §Worker Installer).
	poisonMu sync.Mutex
	poisoned map[string]bool
}

// New constructs a Worker.  registry and workRoot are required; options
// further configure the worker.
func New(registry *core.BlockRegistry, workRoot string, opts ...Option) *Worker {
	w := &Worker{
		Registry: registry,
		WorkRoot: workRoot,
		executor: coreExecutor{},
		poisoned: make(map[string]bool),
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// ErrRegistryUnavailable is returned from Run when the configured
// registry handle is nil.  Treated as an infrastructure failure.
var ErrRegistryUnavailable = errors.New("block registry unavailable")

// Run drives a single job to completion.
//
// See the package doc for the distinction between block failures
// (non-nil result + nil error) and infrastructure failures
// (zero result + non-nil error).
func (w *Worker) Run(ctx context.Context, job spade.Job) (core.WorkerResult, error) {
	if w.Registry == nil {
		return core.WorkerResult{}, ErrRegistryUnavailable
	}
	if err := ctx.Err(); err != nil {
		return core.WorkerResult{}, err
	}

	result := core.WorkerResult{
		InvocationID: job.Assignment.InvocationID,
		PipelineID:   job.Assignment.PipelineID,
		ExitCode:     -1,
	}

	// 1. Registry lookup by block name (and pinned collection version, if any).
	// A miss becomes a registry fetch when an installer is configured and the
	// assignment pins a version (Option A); otherwise it is a terminal block
	// failure. Fetch outcomes split by the installer's failure taxonomy:
	//   - permanent rejection (bad signature / hash / recalled) → block failure
	//     plus a sticky poison marker so we do not re-hit the registry;
	//   - transient (registry unreachable / 5xx) → infrastructure failure (nack →
	//     redeliver) so a registry blip does not mass-fail pipelines.
	version := job.Assignment.CollectionVersion
	entry, err := w.Registry.LookupBlock(job.Assignment.BlockName, version)
	if err != nil && w.installer != nil && version != "" {
		collection := core.CollectionNameFromBlockID(job.Assignment.BlockName)
		if w.isPoisoned(collection, version) {
			result.Status = core.ExecutionStatusError
			result.Error = fmt.Sprintf("block %s: artifact %s/%s previously rejected (poisoned)", job.Assignment.BlockName, collection, version)
			return result, nil
		}
		if ierr := w.installer.Install(ctx, collection, version); ierr != nil {
			if installer.IsRejected(ierr) {
				w.poison(collection, version)
				result.Status = core.ExecutionStatusError
				result.Error = fmt.Sprintf("installing %s/%s: %v", collection, version, ierr)
				return result, nil
			}
			// Transient / infrastructure failure — do not publish a result.
			return core.WorkerResult{}, fmt.Errorf("installing %s/%s: %w", collection, version, ierr)
		}
		entry, err = w.Registry.LookupBlock(job.Assignment.BlockName, version)
	}
	if err != nil {
		result.Status = core.ExecutionStatusError
		result.Error = fmt.Sprintf("block not installed: %s: %v", job.Assignment.BlockName, err)
		return result, nil
	}

	// 1b. Recall / freshness re-check on a hit for a registry-installed block
	// whose last verification is stale (worker.md §Recall). A recalled version
	// refuses to run (block failure with a recalled reason; the installer evicts
	// it). A transient re-check failure is non-fatal: we proceed best-effort with
	// the already-installed, previously-verified block rather than stall pipelines
	// on a registry blip.
	if w.installer != nil && w.freshness > 0 && entry.Source == core.InstallSourceRegistry &&
		time.Since(entry.LastVerifiedAt) > w.freshness {
		if rerr := w.installer.Recheck(ctx, entry.CollectionName, entry.CollectionVersion); rerr != nil && installer.IsRejected(rerr) {
			result.Status = core.ExecutionStatusError
			result.Error = fmt.Sprintf("block %s recalled: %v", job.Assignment.BlockName, rerr)
			return result, nil
		}
	}

	// 2. Load the manifest from the registered install path.  This is the
	// authoritative manifest for hash-verified execution.  The Job's
	// Manifests map is used only for dependency resolution.
	manifest, err := core.LoadManifestForEntry(*entry)
	if err != nil {
		result.Status = core.ExecutionStatusError
		result.Error = fmt.Sprintf("loading manifest for %s: %v", job.Assignment.BlockName, err)
		return result, nil
	}

	// 3. Build the core.BlockInvocation from the job payload.
	inv, err := spade.InvocationFromJob(job)
	if err != nil {
		result.Status = core.ExecutionStatusError
		result.Error = err.Error()
		return result, nil
	}

	// 4. Resolve the pipeline directory.  We use WorkDir if the scheduler
	// supplied it; otherwise derive from WorkRoot + pipeline ID.
	pipelineDir := job.Assignment.WorkDir
	if pipelineDir == "" {
		if w.WorkRoot == "" {
			return core.WorkerResult{}, fmt.Errorf("no work root configured and job carries no WorkDir")
		}
		pipelineDir = filepath.Join(w.WorkRoot, job.Assignment.PipelineID.String())
	}
	if err := os.MkdirAll(pipelineDir, 0777); err != nil {
		// Cannot create the shared work dir → infrastructure failure.
		return core.WorkerResult{}, fmt.Errorf("creating pipeline dir %s: %w", pipelineDir, err)
	}

	// 5. Dependency manifests for symlink-time resolution.  An error here
	// means the scheduler shipped an incomplete Job — protocol error.
	depManifests, err := spade.DependencyManifests(job, inv)
	if err != nil {
		result.Status = core.ExecutionStatusError
		result.Error = err.Error()
		return result, nil
	}

	// 6. Execute the block.  core.Execute handles:
	//      - CreateBlockDirectory (params.yaml, inputs/, outputs/, logs/)
	//      - Writing invocation.yaml
	//      - Hash verification of the installed block
	//      - Running under isolate with language-specific binds
	//      - Capturing stdout/stderr to logs/
	//      - Map block expansion manifest read-back
	//
	// NOTE: the current core.Execute signature does not yet propagate
	// context cancellation into the subprocess.  When ctx is cancelled
	// mid-execution, the in-flight isolate call will complete and only
	// then observe the cancellation when the caller next checks ctx.Err.
	// This is acceptable under the spec's "worker-failure" convention —
	// the binary simply does not ack, and the broker redelivers.
	_ = ctx // captured for future context-aware core.Execute

	// 7. Pre-resolve inputs and populate symlinks.  core.Execute itself
	// does not call SetupInputSymlinks — that is the orchestration
	// layer's job.  We compute resolved inputs once and wire up the
	// invocation directory before dispatching the subprocess.
	pipelineBlock, err := findPipelineBlock(job.Pipeline, inv.Id)
	if err != nil {
		result.Status = core.ExecutionStatusError
		result.Error = err.Error()
		return result, nil
	}
	resolved, err := core.ResolveInputs(pipelineBlock, depManifests, manifest)
	if err != nil {
		result.Status = core.ExecutionStatusError
		result.Error = fmt.Sprintf("resolving inputs: %v", err)
		return result, nil
	}

	// core.Execute creates the invocation directory itself, so we
	// defer symlink setup until after Execute has called
	// CreateBlockDirectory and WriteParamsYAML.  To achieve that in
	// the current core API we pre-create the directory + symlinks
	// here, then call Execute — Execute's CreateBlockDirectory is
	// idempotent (MkdirAll) and WriteParamsYAML overwrites.
	if err := core.CreateBlockDirectory(inv.InvocationID(), pipelineDir); err != nil {
		return core.WorkerResult{}, fmt.Errorf("creating block directory: %w", err)
	}
	invWorkDir := filepath.Join(pipelineDir, inv.InvocationID())
	// Context depths let symlink setup pick the right instance directory
	// for nested map/reduce dependencies (nil ⇒ legacy depth-1 probing).
	depDepths := core.DependencyDepths(job.Pipeline, job.Manifests)
	if err := core.SetupInputSymlinks(invWorkDir, resolved, pipelineDir, inv, manifest, depManifests, depDepths); err != nil {
		result.Status = core.ExecutionStatusError
		result.Error = fmt.Sprintf("setting up input symlinks: %v", err)
		return result, nil
	}

	// 8. Resolve the block's declared secrets from the KMS (authorized by the
	// job's capability token), then dispatch execution. A resolve failure is a
	// worker-side failure: return an error so the caller does not ack and the
	// broker redelivers (spec/worker.md §Error Handling). Values are never logged.
	secrets, err := w.resolveSecrets(ctx, job, pipelineBlock)
	if err != nil {
		return core.WorkerResult{}, err
	}
	execResult, err := w.executor.Execute(inv, pipelineDir, manifest, *entry, w.Registry, secrets)
	if err != nil {
		// core.Execute treats subprocess exec errors as both err and
		// Status=Error.  We classify any such non-nil err as a block
		// failure unless it's the integrity-check error (which core
		// returns as err + Status=Error) — propagate as block failure
		// so the broker acks.
		result.Status = core.ExecutionStatusError
		result.Error = execResult.Error
		if result.Error == "" {
			result.Error = err.Error()
		}
		result.LogsPath = execResult.LogsPath
		return result, nil
	}

	// 9. Convert BlockInvocationResult → WorkerResult.
	result.Status = execResult.Status
	result.Error = execResult.Error
	result.ExitCode = execResult.ExitCode
	result.LogsPath = execResult.LogsPath
	result.Expansion = execResult.Expansion

	// 10. Collect output hashes.  CollectOutputs is safe to call after
	// a successful execution; for a failed one, it returns whatever
	// partial outputs exist (useful for debugging but not authoritative).
	if result.Status == core.ExecutionStatusComplete || result.Status == core.ExecutionStatusMap {
		outHashes, hashErr := core.CollectOutputs(invWorkDir)
		if hashErr == nil {
			result.OutputHashes = outHashes
		}
	}

	return result, nil
}

// poison marks a collection/version as permanently rejected.
func (w *Worker) poison(collection, version string) {
	w.poisonMu.Lock()
	defer w.poisonMu.Unlock()
	w.poisoned[collection+"/"+version] = true
}

// isPoisoned reports whether a collection/version was previously rejected.
func (w *Worker) isPoisoned(collection, version string) bool {
	w.poisonMu.Lock()
	defer w.poisonMu.Unlock()
	return w.poisoned[collection+"/"+version]
}

// resolveSecrets resolves the secrets the block declared into a map of the
// block's logical names to values, for injection into the sandbox. Returns nil
// when the block declares no secrets. Any failure is a worker-side error (the
// caller does not ack; the job redelivers). Secret values are never logged.
func (w *Worker) resolveSecrets(ctx context.Context, job spade.Job, pb core.PipelineBlock) (map[string]string, error) {
	if len(pb.Secrets) == 0 {
		return nil, nil
	}
	if w.secretResolver == nil {
		return nil, fmt.Errorf("block %s declares secrets but no KMS resolver is configured", pb.Name)
	}

	// The stored-secret names are the values of the block's secrets map. Dedupe
	// and sort so the resolve request is deterministic.
	seen := map[string]bool{}
	stored := make([]string, 0, len(pb.Secrets))
	for _, storedName := range pb.Secrets {
		if !seen[storedName] {
			seen[storedName] = true
			stored = append(stored, storedName)
		}
	}
	sort.Strings(stored)

	resolved, err := w.secretResolver.Resolve(ctx, job.CapabilityToken, stored)
	if err != nil {
		return nil, fmt.Errorf("resolving secrets from KMS: %w", err)
	}

	// Re-key from stored names to the block's logical names (what get_secret uses).
	out := make(map[string]string, len(pb.Secrets))
	for logical, storedName := range pb.Secrets {
		v, ok := resolved[storedName]
		if !ok {
			return nil, fmt.Errorf("KMS did not return declared secret %q", storedName)
		}
		out[logical] = v
	}
	return out, nil
}

// findPipelineBlock returns the PipelineBlock matching the given UUID.
func findPipelineBlock(p core.Pipeline, id any) (core.PipelineBlock, error) {
	for _, pb := range p.Blocks {
		if pb.Id == id {
			return pb, nil
		}
	}
	return core.PipelineBlock{}, fmt.Errorf("pipeline block %v not found", id)
}
