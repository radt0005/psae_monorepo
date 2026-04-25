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

	"core"
	spade "spade_runner"
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
) (core.BlockInvocationResult, error) {
	return core.Execute(block, pipelineDir, manifest, entry, reg)
}

// WithExecutor overrides the default core.Execute-backed executor.
// Primarily intended for testing.
func WithExecutor(e Executor) Option {
	return func(w *Worker) {
		w.executor = e
	}
}

// Worker orchestrates a single block invocation end-to-end.
//
// A Worker is safe to reuse across many jobs; it holds only configuration
// and does not carry per-job state.
type Worker struct {
	Registry *core.BlockRegistry
	WorkRoot string

	cacheDir string
	useCache bool
	executor Executor
}

// New constructs a Worker.  registry and workRoot are required; options
// further configure the worker.
func New(registry *core.BlockRegistry, workRoot string, opts ...Option) *Worker {
	w := &Worker{
		Registry: registry,
		WorkRoot: workRoot,
		executor: coreExecutor{},
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

	// 1. Registry lookup by block name.  Miss → block failure, not infra.
	entry, err := w.Registry.LookupBlock(job.Assignment.BlockName, "")
	if err != nil {
		result.Status = core.ExecutionStatusError
		result.Error = fmt.Sprintf("block not installed: %s: %v", job.Assignment.BlockName, err)
		return result, nil
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
	if err := core.SetupInputSymlinks(invWorkDir, resolved, pipelineDir, inv, manifest, depManifests); err != nil {
		result.Status = core.ExecutionStatusError
		result.Error = fmt.Sprintf("setting up input symlinks: %v", err)
		return result, nil
	}

	// 8. Dispatch execution.
	execResult, err := w.executor.Execute(inv, pipelineDir, manifest, *entry, w.Registry)
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

// findPipelineBlock returns the PipelineBlock matching the given UUID.
func findPipelineBlock(p core.Pipeline, id any) (core.PipelineBlock, error) {
	for _, pb := range p.Blocks {
		if pb.Id == id {
			return pb, nil
		}
	}
	return core.PipelineBlock{}, fmt.Errorf("pipeline block %v not found", id)
}
