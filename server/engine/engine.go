package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"core"
	spade "spade_runner"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"spade_server/broker"
	"spade_server/store"
)

// Engine is the heart of the scheduling server.  It owns the in-memory
// MultiTenantScheduler, persists every state change to the store, and
// drives dispatch through the broker.
type Engine struct {
	store     store.Store
	publisher broker.JobPublisher
	manifests ManifestProvider
	logger    *slog.Logger

	mu    sync.Mutex
	sched *core.MultiTenantScheduler

	// ready is signaled whenever blocks may have become executable.
	// Capacity 1 so a flurry of signals coalesces into a single
	// dispatch sweep.
	ready chan struct{}
}

// New constructs an Engine with the given dependencies.  The returned
// engine has an empty scheduler; callers should call Recover before
// starting the loops if they want to rebuild state from the store.
func New(s store.Store, pub broker.JobPublisher, mp ManifestProvider, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{
		store:     s,
		publisher: pub,
		manifests: mp,
		logger:    logger,
		sched: &core.MultiTenantScheduler{
			Pipelines:         map[uuid.UUID]core.Pipeline{},
			Schedulers:        map[uuid.UUID]*core.SinglePipelineScheduler{},
			Workers:           map[uuid.UUID]core.Worker{},
			CurrentExecutions: map[uuid.UUID]core.BlockInvocation{},
		},
		ready: make(chan struct{}, 1),
	}
}

// UpdatePublisher swaps the broker JobPublisher in place.  Used by the
// broker reconnect loop so each reconnect produces a fresh publisher
// without needing to recreate the entire Engine.
func (e *Engine) UpdatePublisher(p broker.JobPublisher) {
	e.mu.Lock()
	e.publisher = p
	e.mu.Unlock()
}

// signalReady wakes the dispatch loop.  Non-blocking — extra signals
// while the loop is mid-sweep coalesce.
func (e *Engine) signalReady() {
	select {
	case e.ready <- struct{}{}:
	default:
	}
}

// SubmitPipeline validates, persists, and registers a new pipeline.
// On success the source blocks are queued for dispatch.  The pipeline
// is passed by pointer so a newly-minted UUIDv7 is visible to the
// caller after submission.
func (e *Engine) SubmitPipeline(ctx context.Context, pp *core.Pipeline, yamlBody []byte, submitter string) error {
	if pp == nil {
		return fmt.Errorf("nil pipeline")
	}
	p := *pp
	defer func() { *pp = p }()
	if p.Id == uuid.Nil {
		// Mint a UUIDv7 if the caller didn't supply one.  pipeline.md
		// §10 says the pipeline ID is generated at submission time.
		newID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generating pipeline id: %w", err)
		}
		p.Id = newID
	}

	// Validate against the configured manifest provider.
	manifestMap := e.manifestMap()
	if errs := core.ValidatePipeline(p, manifestMap); len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}

	// If the caller didn't supply YAML, marshal what we have.
	if len(yamlBody) == 0 {
		bytes, err := yaml.Marshal(&p)
		if err != nil {
			return fmt.Errorf("marshaling pipeline yaml: %w", err)
		}
		yamlBody = bytes
	}

	rec := store.PipelineRecord{
		ID:              p.Id,
		Name:            p.Name,
		Version:         p.Version,
		Description:     p.Description,
		YAML:            string(yamlBody),
		Status:          store.PipelineRunning,
		SubmittedAt:     time.Now().UTC(),
		SubmitterUserID: submitter,
	}
	if err := e.store.InsertPipeline(ctx, rec); err != nil {
		return err
	}
	if err := e.store.AppendEvent(ctx, store.PipelineEvent{
		PipelineID:  p.Id,
		EventType:   store.EventSubmitted,
		PayloadJSON: string(yamlBody),
	}); err != nil {
		return err
	}

	// Pre-persist a pending invocation row for every block so that the
	// status snapshot is meaningful even before anything dispatches.
	for _, pb := range p.Blocks {
		invID := pb.Id.String()
		if err := e.store.UpsertInvocation(ctx, store.InvocationRecord{
			ID:         invID,
			PipelineID: p.Id,
			BlockID:    pb.Id,
			BlockName:  pb.Name,
			Status:     store.InvocationPending,
		}); err != nil {
			return err
		}
	}

	e.mu.Lock()
	if err := e.sched.AddPipeline(p); err != nil {
		e.mu.Unlock()
		return err
	}
	// Pre-populate per-scheduler manifest map so map/reduce detection
	// works for pipelines containing map blocks.
	if ps := e.sched.Schedulers[p.Id]; ps != nil {
		ps.Manifests = manifestMap
		_ = ps.IdentifyMapContexts()
	}
	e.mu.Unlock()
	e.signalReady()
	return nil
}

// CancelPipeline drops pending and executable blocks for the pipeline
// and marks it cancelled in the store.  In-flight invocations are not
// recalled; their results will be dropped by the duplicate-detection
// path in the result loop.
func (e *Engine) CancelPipeline(ctx context.Context, id uuid.UUID) error {
	e.mu.Lock()
	err := e.sched.CancelPipeline(id)
	e.mu.Unlock()
	if err != nil {
		// CancelPipeline returns the scheduler's "not found" error.
		// Even when the scheduler doesn't know about it, the store
		// might — best-effort update.
	}
	if upderr := e.store.UpdatePipelineStatus(ctx, id, store.PipelineCancelled); upderr != nil {
		if errors.Is(upderr, store.ErrNotFound) {
			return upderr
		}
		return fmt.Errorf("updating pipeline status: %w", upderr)
	}
	_ = e.store.AppendEvent(ctx, store.PipelineEvent{
		PipelineID: id,
		EventType:  store.EventPipelineCancelled,
	})
	return nil
}

// PipelineStatus returns the snapshot for one pipeline, merged with the
// persisted header.
func (e *Engine) PipelineStatus(ctx context.Context, id uuid.UUID) (PipelineStatusView, error) {
	rec, err := e.store.LoadPipeline(ctx, id)
	if err != nil {
		return PipelineStatusView{}, err
	}
	e.mu.Lock()
	snap, ok := e.sched.Snapshot(id)
	e.mu.Unlock()
	view := PipelineStatusView{
		ID:          rec.ID,
		Name:        rec.Name,
		Version:     rec.Version,
		Status:      rec.Status,
		SubmittedAt: rec.SubmittedAt,
		CompletedAt: rec.CompletedAt,
	}
	if ok {
		view.Blocks = snap.Blocks
		view.Cancelled = snap.Cancelled
		view.Complete = snap.Complete
		view.Failed = snap.Failed
	}
	return view, nil
}

// PipelineStatusView combines persisted header info with the in-memory
// per-block snapshot.
type PipelineStatusView struct {
	ID          uuid.UUID
	Name        string
	Version     string
	Status      store.PipelineStatus
	SubmittedAt time.Time
	CompletedAt *time.Time
	Cancelled   bool
	Complete    bool
	Failed      bool
	Blocks      []core.BlockSnapshot
}

// ValidationError carries the multi-error from core.ValidatePipeline.
type ValidationError struct{ Errors []error }

// Error renders the contained errors as a single comma-joined string.
func (v *ValidationError) Error() string {
	if v == nil || len(v.Errors) == 0 {
		return "pipeline validation failed"
	}
	if len(v.Errors) == 1 {
		return v.Errors[0].Error()
	}
	msg := "pipeline validation failed: "
	for i, e := range v.Errors {
		if i > 0 {
			msg += "; "
		}
		msg += e.Error()
	}
	return msg
}

// manifestMap snapshots whatever manifests the provider can deliver.
// MapManifestProvider implements an All() method for this; if the
// provider does not, we degrade to lazy lookups during validation.
func (e *Engine) manifestMap() map[string]core.BlockManifest {
	if mm, ok := e.manifests.(interface {
		All() map[string]core.BlockManifest
	}); ok {
		return mm.All()
	}
	return map[string]core.BlockManifest{}
}

// Run starts the dispatch and result loops and blocks until ctx is
// done or one of the loops returns a fatal error.  The result consumer
// is owned by the caller; engine does not close it.
func (e *Engine) Run(ctx context.Context, consumer broker.ResultConsumer) error {
	// Trigger initial sweep in case any pre-recovered blocks are ready.
	e.signalReady()

	errCh := make(chan error, 2)
	go func() { errCh <- e.dispatchLoop(ctx) }()
	go func() { errCh <- e.resultLoop(ctx, consumer) }()

	// Wait for first error or ctx done.
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	case <-ctx.Done():
	}
	return ctx.Err()
}

// Close shuts the engine down.  The store and broker are owned by the
// caller; only engine-private resources are released here.
func (e *Engine) Close() error { return nil }

// Recover rebuilds in-memory scheduler state from the store.
// scheduler.md §State Management requires the scheduler to be
// reconstructable from durable storage at any time.
func (e *Engine) Recover(ctx context.Context) error {
	active, err := e.store.LoadActivePipelinesForRestart(ctx)
	if err != nil {
		return fmt.Errorf("loading active pipelines: %w", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	manifestMap := e.manifestMap()
	for _, ap := range active {
		var p core.Pipeline
		if err := yaml.Unmarshal([]byte(ap.Pipeline.YAML), &p); err != nil {
			e.logger.Error("recover: failed to parse persisted pipeline yaml",
				"pipeline_id", ap.Pipeline.ID, "err", err)
			continue
		}
		if p.Id == uuid.Nil {
			p.Id = ap.Pipeline.ID
		}
		// Translate persisted invocation rows into BlockInvocationResults
		// for the parts that completed.  We only replay terminal rows;
		// dispatched-but-unacked invocations are recovered by the
		// broker's redelivery.
		var replay []core.BlockInvocationResult
		for _, inv := range ap.Invocations {
			switch inv.Status {
			case store.InvocationComplete:
				replay = append(replay, recordToResult(inv, ap.Pipeline.ID, core.ExecutionStatusComplete))
			case store.InvocationError:
				replay = append(replay, recordToResult(inv, ap.Pipeline.ID, core.ExecutionStatusError))
			case store.InvocationMap:
				replay = append(replay, recordToResult(inv, ap.Pipeline.ID, core.ExecutionStatusMap))
			case store.InvocationReduce:
				replay = append(replay, recordToResult(inv, ap.Pipeline.ID, core.ExecutionStatusReduce))
			}
		}
		if err := e.sched.Rehydrate(p, replay); err != nil {
			e.logger.Error("recover: rehydrate failed", "pipeline_id", ap.Pipeline.ID, "err", err)
			continue
		}
		if ps := e.sched.Schedulers[p.Id]; ps != nil {
			ps.Manifests = manifestMap
			_ = ps.IdentifyMapContexts()
		}
	}
	return nil
}

func recordToResult(inv store.InvocationRecord, pid uuid.UUID, status core.ExecutionStatus) core.BlockInvocationResult {
	r := core.BlockInvocationResult{
		Id:         inv.BlockID,
		PipelineId: pid,
		Status:     status,
		ExitCode:   inv.ExitCode,
		LogsPath:   inv.LogsPath,
		Error:      inv.ErrorMessage,
	}
	if inv.ExpansionJSON != "" {
		var ex core.ExpansionManifest
		if err := json.Unmarshal([]byte(inv.ExpansionJSON), &ex); err == nil {
			r.Expansion = &ex
		}
	}
	return r
}

// snapshotSchedulerLocked returns a copy of every scheduler so callers
// can inspect state without holding the mutex for long.  Caller must
// hold e.mu.
func (e *Engine) snapshotSchedulerLocked() map[uuid.UUID]*core.SinglePipelineScheduler {
	out := make(map[uuid.UUID]*core.SinglePipelineScheduler, len(e.sched.Schedulers))
	for k, v := range e.sched.Schedulers {
		out[k] = v
	}
	return out
}

// dispatchLoop drains executable invocations from the scheduler and
// publishes them as Job messages.  Returns ctx.Err() when ctx is done.
func (e *Engine) dispatchLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-e.ready:
		}
		if err := e.dispatchSweep(ctx); err != nil {
			e.logger.Error("dispatch sweep failed", "err", err)
			// Don't return — the loop continues so transient
			// failures don't kill the engine.
		}
	}
}

func (e *Engine) dispatchSweep(ctx context.Context) error {
	e.mu.Lock()
	drained := e.sched.Drain()
	scheds := e.snapshotSchedulerLocked()
	e.mu.Unlock()

	if len(drained) == 0 {
		return nil
	}

	for _, inv := range drained {
		ps, ok := scheds[inv.PipelineId]
		if !ok || ps == nil {
			e.logger.Warn("drained invocation for unknown pipeline", "pipeline_id", inv.PipelineId)
			continue
		}
		// Build the outgoing Job: assemble manifests for this block
		// and every direct dependency.
		manifests := e.manifestMap()
		blockManifests := map[string]core.BlockManifest{}
		if m, ok := manifests[inv.BlockId]; ok {
			blockManifests[inv.BlockId] = m
		}
		for _, ref := range inv.Inputs {
			var depID uuid.UUID
			if ref.Block != nil {
				depID = *ref.Block
			} else {
				depID = ref.ID
			}
			if depID == uuid.Nil {
				continue
			}
			for _, pb := range ps.Pipeline.Blocks {
				if pb.Id == depID {
					if m, ok := manifests[pb.Name]; ok {
						blockManifests[pb.Name] = m
					}
					break
				}
			}
		}

		job := spade.BuildJobForInvocation(inv, ps.Pipeline, blockManifests, "")
		if err := e.publisher.Publish(ctx, job); err != nil {
			// Put the invocation back so it tries again on the next
			// reconnect/sweep.  Re-enqueue at the head of executable
			// for this scheduler.
			e.mu.Lock()
			ps := e.sched.Schedulers[inv.PipelineId]
			if ps != nil {
				ps.ExecutableBlocks = append([]core.BlockInvocation{inv}, ps.ExecutableBlocks...)
			}
			e.mu.Unlock()
			return fmt.Errorf("publishing job %s: %w", inv.InvocationID(), err)
		}
		now := time.Now().UTC()
		invID := inv.InvocationID()
		dispatched := store.InvocationRecord{
			ID:           invID,
			PipelineID:   inv.PipelineId,
			BlockID:      inv.Id,
			BlockName:    inv.BlockId,
			MapIndex:     inv.MapIndex,
			Status:       store.InvocationDispatched,
			DispatchedAt: &now,
		}
		if err := e.store.UpsertInvocation(ctx, dispatched); err != nil {
			e.logger.Error("persisting dispatched invocation", "id", invID, "err", err)
		}
		_ = e.store.AppendEvent(ctx, store.PipelineEvent{
			PipelineID:   inv.PipelineId,
			InvocationID: invID,
			EventType:    store.EventBlockDispatched,
		})
	}
	return nil
}

// resultLoop consumes from spade.results, applies each result to the
// scheduler, persists the transition, and signals dispatch.
func (e *Engine) resultLoop(ctx context.Context, consumer broker.ResultConsumer) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		delivery, err := consumer.Next(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return fmt.Errorf("consuming result: %w", err)
		}
		if delivery.Result.InvocationID == "" {
			e.logger.Warn("malformed result payload, discarding", "bytes", len(delivery.RawBody))
			if delivery.Nack != nil {
				_ = delivery.Nack(ctx, false)
			}
			continue
		}
		if err := e.applyResult(ctx, delivery.Result); err != nil {
			e.logger.Error("applying result failed", "id", delivery.Result.InvocationID, "err", err)
			if delivery.Nack != nil {
				_ = delivery.Nack(ctx, false)
			}
			continue
		}
		if delivery.Ack != nil {
			_ = delivery.Ack(ctx)
		}
	}
}

func (e *Engine) applyResult(ctx context.Context, wr core.WorkerResult) error {
	uuidPart, mapIdx, _ := spade.ParseInvocationID(wr.InvocationID)

	e.mu.Lock()
	if e.sched.IsAlreadyProcessed(wr.InvocationID) {
		e.mu.Unlock()
		e.logger.Info("duplicate result ignored", "id", wr.InvocationID)
		return nil
	}
	result := core.WorkerResultToInvocationResult(wr)
	if err := e.sched.Update(uuidPart, result); err != nil {
		e.mu.Unlock()
		return fmt.Errorf("scheduler update: %w", err)
	}
	// After Update, mapped fan-out may have created new entries in
	// PendingBlocks / ExecutableBlocks.  Capture which extra rows
	// need to be persisted while we still hold the lock.
	pendingRows := e.collectPendingRowsLocked(wr.PipelineID, wr.Status, wr.Expansion)
	failure := wr.Status == core.ExecutionStatusError
	complete := false
	if !failure {
		complete = e.pipelineCompleteLocked(wr.PipelineID)
	}
	e.mu.Unlock()

	now := time.Now().UTC()
	invID := wr.InvocationID
	rec := store.InvocationRecord{
		ID:           invID,
		PipelineID:   wr.PipelineID,
		BlockID:      result.Id,
		BlockName:    "",
		MapIndex:     mapIdx,
		Status:       statusFromWorker(wr.Status),
		CompletedAt:  &now,
		ExitCode:     wr.ExitCode,
		LogsPath:     wr.LogsPath,
		ErrorMessage: wr.Error,
	}
	// Look up block name from the persisted record so we don't lose
	// it on the terminal write.
	if existing, err := e.store.LoadInvocation(ctx, invID); err == nil {
		rec.BlockName = existing.BlockName
	}
	if len(wr.OutputHashes) > 0 {
		if hb, err := json.Marshal(wr.OutputHashes); err == nil {
			rec.OutputHashesJSON = string(hb)
		}
	}
	if wr.Expansion != nil {
		if eb, err := json.Marshal(wr.Expansion); err == nil {
			rec.ExpansionJSON = string(eb)
		}
	}
	if err := e.store.UpsertInvocation(ctx, rec); err != nil {
		return err
	}

	// Persist the newly-pending mapped invocations.
	for _, prow := range pendingRows {
		_ = e.store.UpsertInvocation(ctx, prow)
	}

	evt := store.PipelineEvent{
		PipelineID:   wr.PipelineID,
		InvocationID: invID,
		EventType:    store.EventBlockCompleted,
	}
	if failure {
		evt.EventType = store.EventBlockFailed
	}
	_ = e.store.AppendEvent(ctx, evt)

	if failure {
		_ = e.store.UpdatePipelineStatus(ctx, wr.PipelineID, store.PipelineFailed)
		_ = e.store.AppendEvent(ctx, store.PipelineEvent{
			PipelineID: wr.PipelineID,
			EventType:  store.EventPipelineFailed,
		})
	} else if complete {
		_ = e.store.UpdatePipelineStatus(ctx, wr.PipelineID, store.PipelineComplete)
		_ = e.store.AppendEvent(ctx, store.PipelineEvent{
			PipelineID: wr.PipelineID,
			EventType:  store.EventPipelineCompleted,
		})
	}

	e.signalReady()
	return nil
}

// statusFromWorker maps a WorkerResult.Status to the persisted form.
func statusFromWorker(status core.ExecutionStatus) store.InvocationStatus {
	switch status {
	case core.ExecutionStatusComplete:
		return store.InvocationComplete
	case core.ExecutionStatusError:
		return store.InvocationError
	case core.ExecutionStatusMap:
		return store.InvocationMap
	case core.ExecutionStatusReduce:
		return store.InvocationReduce
	default:
		return store.InvocationStatus(status)
	}
}

// pipelineCompleteLocked checks whether every block in the pipeline has
// reached a terminal state.  Caller must hold e.mu.
func (e *Engine) pipelineCompleteLocked(pid uuid.UUID) bool {
	ps, ok := e.sched.Schedulers[pid]
	if !ok || ps == nil {
		return false
	}
	if ps.Cancelled {
		return false
	}
	if len(ps.PendingBlocks) > 0 || len(ps.ExecutableBlocks) > 0 {
		return false
	}
	return true
}

// collectPendingRowsLocked enumerates the persisted rows that ought to
// exist after the most recent Update, in particular new rows for each
// mapped fan-out.  Caller must hold e.mu.
func (e *Engine) collectPendingRowsLocked(pid uuid.UUID, status core.ExecutionStatus, exp *core.ExpansionManifest) []store.InvocationRecord {
	if status != core.ExecutionStatusMap || exp == nil {
		return nil
	}
	ps, ok := e.sched.Schedulers[pid]
	if !ok || ps == nil {
		return nil
	}
	var out []store.InvocationRecord
	for _, inv := range ps.ExecutableBlocks {
		if inv.MapIndex == nil {
			continue
		}
		out = append(out, store.InvocationRecord{
			ID:         inv.InvocationID(),
			PipelineID: inv.PipelineId,
			BlockID:    inv.Id,
			BlockName:  inv.BlockId,
			MapIndex:   inv.MapIndex,
			Status:     store.InvocationReady,
		})
	}
	for _, inv := range ps.PendingBlocks {
		if inv.MapIndex == nil {
			continue
		}
		out = append(out, store.InvocationRecord{
			ID:         inv.InvocationID(),
			PipelineID: inv.PipelineId,
			BlockID:    inv.Id,
			BlockName:  inv.BlockId,
			MapIndex:   inv.MapIndex,
			Status:     store.InvocationPending,
		})
	}
	return out
}

