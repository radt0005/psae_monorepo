package core

import (
	"fmt"
	"sort"

	"github.com/google/uuid"
)

// This file holds the scheduler-server-facing helpers added per the
// scheduling server's IMPLEMENTATION_PLAN.md (Phase 3 upstream gaps).
// They are additive: they do not change the semantics of the existing
// SinglePipelineScheduler / MultiTenantScheduler fields and methods.

// BlockSnapshotStatus is the per-block status carried in a snapshot.
type BlockSnapshotStatus string

const (
	BlockSnapshotPending    BlockSnapshotStatus = "pending"
	BlockSnapshotExecutable BlockSnapshotStatus = "executable"
	BlockSnapshotInFlight   BlockSnapshotStatus = "in_flight"
	BlockSnapshotComplete   BlockSnapshotStatus = "complete"
	BlockSnapshotError      BlockSnapshotStatus = "error"
	BlockSnapshotMap        BlockSnapshotStatus = "map"
	BlockSnapshotReduce     BlockSnapshotStatus = "reduce"
)

// BlockSnapshot is a read-only view of one block's state in a pipeline
// snapshot.  Mapped invocations appear under MapInvocations on the map
// block's BlockSnapshot.
type BlockSnapshot struct {
	BlockID        uuid.UUID
	Name           string
	Status         BlockSnapshotStatus
	MapIndices     []int
	MapInvocations []string // invocation IDs of the fan-outs across all instances, if this is a map block
	// InstanceCounts maps each expanded instance of this map block
	// (keyed by index prefix, "" for a top-level map) to its item count.
	// Lets the UI render ragged nested fan-out as aggregates.
	InstanceCounts map[string]int
	Expansion      *ExpansionManifest
	ExitCode       int
	ErrorMessage   string
}

// PipelineSnapshot is the per-pipeline status snapshot used by the HTTP
// API.  Returned from (*SinglePipelineScheduler).Snapshot and
// (*MultiTenantScheduler).Snapshot.
type PipelineSnapshot struct {
	PipelineID uuid.UUID
	Cancelled  bool
	Complete   bool
	Failed     bool
	Blocks     []BlockSnapshot
}

// Snapshot returns a snapshot of the pipeline scheduler's current state.
// The returned value is safe to expose to API callers — it copies every
// piece of state that downstream readers might need.
//
// Implementation notes per Phase 3.2 of the scheduling server's plan.
func (s *SinglePipelineScheduler) Snapshot() PipelineSnapshot {
	snap := PipelineSnapshot{
		PipelineID: s.Pipeline.Id,
		Cancelled:  s.Cancelled,
	}

	// Index executable blocks by invocation ID string so we can label
	// fan-out invocations correctly.
	executable := make(map[string]BlockInvocation)
	for _, inv := range s.ExecutableBlocks {
		executable[inv.InvocationID()] = inv
	}

	// Walk every block declared in the pipeline.
	for _, pb := range s.Pipeline.Blocks {
		bs := BlockSnapshot{
			BlockID: pb.Id,
			Name:    pb.Name,
		}
		// Did this block already complete or error?  Top-level blocks
		// complete under their bare UUID; nested map/reduce instances
		// complete under per-instance IDs and are surfaced via
		// MapInvocations on the enclosing map block instead.
		if res, ok := s.CompletedBlocks[pb.Id.String()]; ok {
			switch res.Status {
			case ExecutionStatusComplete:
				bs.Status = BlockSnapshotComplete
			case ExecutionStatusError:
				bs.Status = BlockSnapshotError
			case ExecutionStatusMap:
				bs.Status = BlockSnapshotMap
			case ExecutionStatusReduce:
				bs.Status = BlockSnapshotReduce
			default:
				bs.Status = BlockSnapshotComplete
			}
			bs.ExitCode = res.ExitCode
			bs.ErrorMessage = res.Error
			bs.Expansion = res.Expansion
		} else if _, ok := executable[pb.Id.String()]; ok {
			bs.Status = BlockSnapshotExecutable
		} else if _, ok := s.PendingBlocks[pb.Id.String()]; ok {
			bs.Status = BlockSnapshotPending
		} else {
			// Possibly the block has been expanded into N mapped
			// invocations; we surface that on the map block, not here.
			bs.Status = BlockSnapshotPending
		}

		// If this is a map block with a known context, attach the
		// resolved invocation IDs (across all instances of the context)
		// so the API can show fan-out progress.
		if ctx, ok := s.MapContexts[pb.Id]; ok && ctx != nil {
			prefixes := make([]string, 0, len(ctx.Expansions))
			for p := range ctx.Expansions {
				prefixes = append(prefixes, p)
			}
			sort.Strings(prefixes)
			if len(prefixes) > 0 {
				bs.InstanceCounts = make(map[string]int, len(prefixes))
			}
			for _, p := range prefixes {
				bs.InstanceCounts[p] = len(ctx.Expansions[p])
				base := pb.Id.String()
				if p != "" {
					base += "." + p
				}
				for i := range ctx.Expansions[p] {
					bs.MapInvocations = append(bs.MapInvocations, fmt.Sprintf("%s.%d", base, i))
				}
			}
		}

		snap.Blocks = append(snap.Blocks, bs)
	}

	// Detect terminal states from the scheduler's own bookkeeping.
	if s.Cancelled {
		// Cancelled-with-failure is when there is at least one error
		// result in CompletedBlocks; cancelled-with-cancel is otherwise.
		for _, res := range s.CompletedBlocks {
			if res.Status == ExecutionStatusError {
				snap.Failed = true
				break
			}
		}
	}
	if !snap.Cancelled && len(s.PendingBlocks) == 0 && len(s.ExecutableBlocks) == 0 {
		snap.Complete = true
	}

	return snap
}

// --- MultiTenantScheduler helpers ---

// Drain returns every currently executable BlockInvocation across every
// pipeline and removes them from the per-pipeline ExecutableBlocks
// queues.  The caller is then responsible for dispatching each one.
//
// Drain does NOT record the returned invocations in CurrentExecutions —
// that field is keyed by UUID and is preserved for legacy callers.  The
// server-side engine maintains its own in-flight map keyed by invocation
// ID string so it can correctly handle mapped fan-out.
//
// Implementation notes per Phase 3.1 of the scheduling server's plan.
func (s *MultiTenantScheduler) Drain() []BlockInvocation {
	var out []BlockInvocation
	for _, pid := range s.pipelineOrder {
		ps, ok := s.Schedulers[pid]
		if !ok || ps == nil {
			continue
		}
		if len(ps.ExecutableBlocks) == 0 {
			continue
		}
		out = append(out, ps.ExecutableBlocks...)
		ps.ExecutableBlocks = nil
	}
	return out
}

// Snapshot returns the per-pipeline snapshot identified by id, or false
// if the pipeline is not registered.
func (s *MultiTenantScheduler) Snapshot(id uuid.UUID) (PipelineSnapshot, bool) {
	ps, ok := s.Schedulers[id]
	if !ok || ps == nil {
		return PipelineSnapshot{}, false
	}
	return ps.Snapshot(), true
}

// IsAlreadyProcessed returns true when the scheduler has already
// observed a terminal result for the given invocation ID.  Used by
// scheduler-side consumers to drop duplicate results before mutating
// state, per worker.md §Result reporting ("first result wins").
//
// invocationID is the canonical string form: the block UUID plus one
// ".<index>" component per enclosing map context — exactly the key used
// by CompletedBlocks.
func (s *MultiTenantScheduler) IsAlreadyProcessed(invocationID string) bool {
	for _, ps := range s.Schedulers {
		if ps == nil {
			continue
		}
		if _, ok := ps.CompletedBlocks[invocationID]; ok {
			return true
		}
	}
	return false
}

// Rehydrate registers the pipeline with the multi-tenant scheduler and
// replays the supplied completed results in order so the in-memory
// scheduling state matches what PostgreSQL recorded.  Used by the
// scheduling server's restart-tolerant recovery path (Phase 3.5).
func (s *MultiTenantScheduler) Rehydrate(p Pipeline, completedResults []BlockInvocationResult) error {
	if err := s.AddPipeline(p); err != nil {
		return fmt.Errorf("re-adding pipeline %s: %w", p.Id, err)
	}
	ps, ok := s.Schedulers[p.Id]
	if !ok {
		return fmt.Errorf("scheduler for pipeline %s missing after AddPipeline", p.Id)
	}
	// Replay results.  Update handles the state transitions (including
	// marking blocks complete, halting on error, and fan-out on map).
	// After each terminal result, prune the corresponding entry from
	// ExecutableBlocks — AddPipeline placed source blocks there
	// unconditionally, but a replayed source result means the block is
	// already finished.
	for _, r := range completedResults {
		if err := ps.Update(r); err != nil {
			return fmt.Errorf("replaying result %s: %w", r.InvocationID(), err)
		}
		if r.Status == ExecutionStatusComplete || r.Status == ExecutionStatusError {
			// Prune by full invocation ID so replaying one mapped
			// sibling's result does not drop the others.
			doneID := r.InvocationID()
			filtered := ps.ExecutableBlocks[:0]
			for _, inv := range ps.ExecutableBlocks {
				if inv.InvocationID() == doneID {
					continue
				}
				filtered = append(filtered, inv)
			}
			ps.ExecutableBlocks = filtered
		}
	}
	return nil
}

// WorkerResultToInvocationResult converts a worker-side WorkerResult
// (the message published on spade.results) into a BlockInvocationResult
// suitable for SinglePipelineScheduler.Update.
//
// Implementation notes per Phase 3.8 of the scheduling server's plan.
func WorkerResultToInvocationResult(r WorkerResult) BlockInvocationResult {
	parsed, indices, _ := ParseInvocationID(r.InvocationID)
	return BlockInvocationResult{
		Id:         parsed,
		PipelineId: r.PipelineID,
		MapIndices: indices,
		Status:     r.Status,
		Expansion:  r.Expansion,
		Error:      r.Error,
		ExitCode:   r.ExitCode,
		LogsPath:   r.LogsPath,
	}
}
