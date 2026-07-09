package core

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// MapContext tracks one map block's fan-out context.  The static shape
// (member blocks, closing reduces) comes from the pipeline's ContextTree;
// the Expansions map is filled dynamically as instances of the map block
// complete.  A top-level map has exactly one instance (prefix ""); a map
// nested one level deep has one instance per outer index ("0", "1", …),
// each with its own — possibly different — item count.
type MapContext struct {
	MapBlockID uuid.UUID
	// MappedBlockIDs is the set of blocks that run inside this context at
	// its fan-out depth: standard blocks, nested map blocks, and the
	// reduces that close nested contexts.  Interiors of nested contexts
	// belong to the nested map's own MapContext.
	MappedBlockIDs map[uuid.UUID]bool
	// ReduceBlockIDs are the reduce block(s) that close this context and
	// run at the parent depth.
	ReduceBlockIDs []uuid.UUID
	// Expansions holds the expansion items per context instance, keyed by
	// the map invocation's index prefix ("" for a top-level map).
	Expansions map[string][]ExpansionItem
}

// SinglePipelineScheduler schedules blocks for a single pipeline.
//
// CompletedBlocks and PendingBlocks are keyed by the invocation ID string
// (BlockInvocation.InvocationID()), which encodes both the block UUID and
// the map index vector.  Blocks inside a map context initially sit in
// PendingBlocks under their bare UUID as placeholders; when the enclosing
// map instance expands they are replaced by per-instance invocations.
type SinglePipelineScheduler struct {
	Pipeline         Pipeline
	Cancelled        bool
	ExecutableBlocks []BlockInvocation
	CompletedBlocks  map[string]BlockInvocationResult
	PendingBlocks    map[string]BlockInvocation
	Manifests        map[string]BlockManifest
	MapContexts      map[uuid.UUID]*MapContext
	Tree             *ContextTree
	Graph            DependencyGraph
}

func (s *SinglePipelineScheduler) AddPipeline(p Pipeline) error {
	s.Pipeline = p
	s.Cancelled = false

	// Build dependency graph
	graph, err := BuildDependencyGraph(p)
	if err != nil {
		return fmt.Errorf("building dependency graph: %w", err)
	}
	s.Graph = graph

	for _, item := range p.Blocks {
		invocation := BlockInvocation{
			Id:         item.Id,
			PipelineId: s.Pipeline.Id,
			BlockId:    item.Name,
			Inputs:     item.Inputs,
			Arguments:  item.Args,
		}

		// Source blocks (no inputs) go directly to ExecutableBlocks
		if len(item.Inputs) == 0 {
			s.ExecutableBlocks = append(s.ExecutableBlocks, invocation)
		} else {
			s.PendingBlocks[invocation.InvocationID()] = invocation
		}
	}

	return nil
}

func (s *SinglePipelineScheduler) CancelPipeline(id uuid.UUID) error {
	s.Cancelled = true
	s.PendingBlocks = map[string]BlockInvocation{}
	s.ExecutableBlocks = nil
	return nil
}

func (s *SinglePipelineScheduler) Update(result BlockInvocationResult) error {
	if result.Status == ExecutionStatusError {
		// Halt the pipeline: clear pending and executable, preserve completed for debugging
		s.PendingBlocks = map[string]BlockInvocation{}
		s.ExecutableBlocks = nil
		s.Cancelled = true
		return nil
	}

	switch result.Status {
	case ExecutionStatusComplete, ExecutionStatusReduce:
		invID := result.InvocationID()
		s.CompletedBlocks[invID] = result
		delete(s.PendingBlocks, invID)
		s.promoteReady()
	case ExecutionStatusMap:
		// HandleMap records the expansion for this map instance and fans
		// out the member blocks.  promoteReady then promotes anything
		// whose dependencies are now satisfied — including a reduce that
		// directly follows the map with no intermediate mapped block, and
		// reduces of instances that expanded to zero items.
		s.HandleMap(result)
		s.promoteReady()
	}

	return nil
}

// promoteReady moves every pending invocation whose dependencies are all
// satisfied into ExecutableBlocks.  Placeholder entries for blocks inside
// a map context (bare UUID, no index vector yet) are never promoted; they
// are replaced by per-instance invocations when the enclosing map
// expands.
func (s *SinglePipelineScheduler) promoteReady() {
	var readyKeys []string
	for key, pending := range s.PendingBlocks {
		if s.invocationReady(pending) {
			readyKeys = append(readyKeys, key)
		}
	}
	for _, key := range readyKeys {
		invocation := s.PendingBlocks[key]
		s.ExecutableBlocks = append(s.ExecutableBlocks, invocation)
		delete(s.PendingBlocks, key)
	}
}

// invocationReady reports whether every dependency of the invocation is
// satisfied.  A dependency at context depth e is satisfied by:
//   - e <= depth(inv): the single completed invocation whose index vector
//     is the first e components of inv's vector (e == 0 is a broadcast
//     from the top level, e == depth is a peer in the same instance).
//   - e == depth(inv)+1: inv gathers a full fan-out dimension (it is a
//     reduce closing the dependency's context): every sibling
//     <dep>.<inv indices>.<j> must be complete, where j ranges over the
//     enclosing map instance's expansion items.  An instance with zero
//     items is vacuously satisfied.
//
// Without a context tree (Manifests not provided), all dependencies are
// treated as depth 0 — plain completed-by-UUID matching.
func (s *SinglePipelineScheduler) invocationReady(inv BlockInvocation) bool {
	d := len(inv.MapIndices)

	// Placeholder for a block inside a map context that has not been
	// fanned out yet: never ready.
	if s.Tree != nil && s.Tree.Depth(inv.Id) != d {
		return false
	}

	for _, input := range inv.Inputs {
		var depID uuid.UUID
		if input.Block != nil {
			depID = *input.Block
		} else {
			depID = input.ID
		}
		if depID == uuid.Nil {
			continue
		}

		depDepth := 0
		if s.Tree != nil {
			depDepth = s.Tree.Depth(depID)
		}

		switch {
		case depDepth <= d:
			expected := FormatInvocationID(depID, inv.MapIndices[:depDepth])
			if _, completed := s.CompletedBlocks[expected]; !completed {
				return false
			}
		case depDepth == d+1:
			// The dependency fans out one level deeper than inv: inv
			// gathers all of its siblings in this instance.
			depPath := s.Tree.Paths[depID]
			owner := depPath[len(depPath)-1]
			ctx, ok := s.MapContexts[owner]
			if !ok {
				return false
			}
			prefix := IndexPrefix(inv.MapIndices, d)
			items, expanded := ctx.Expansions[prefix]
			if !expanded {
				return false
			}
			for j := range items {
				sibling := append(append([]int{}, inv.MapIndices...), j)
				expected := FormatInvocationID(depID, sibling)
				if _, completed := s.CompletedBlocks[expected]; !completed {
					return false
				}
			}
		default:
			// More than one level deeper: structurally invalid (caught
			// at validation time); never ready.
			return false
		}
	}
	return true
}

func (s *SinglePipelineScheduler) IsReady() bool {
	return len(s.ExecutableBlocks) > 0
}

func (s *SinglePipelineScheduler) Next() (BlockInvocation, bool, error) {
	if len(s.ExecutableBlocks) > 0 {
		block := s.ExecutableBlocks[0]
		s.ExecutableBlocks = s.ExecutableBlocks[1:]
		return block, false, nil
	}

	if len(s.PendingBlocks) == 0 {
		return BlockInvocation{}, true, nil
	}

	return BlockInvocation{}, false, nil
}

// IdentifyMapContexts builds the pipeline's context tree from the
// manifests and creates one MapContext per map block.  Requires
// s.Manifests to be populated; without manifests it is a no-op (the
// scheduler then treats every block as standard).
func (s *SinglePipelineScheduler) IdentifyMapContexts() error {
	if s.Manifests == nil {
		return nil
	}
	tree, err := BuildContextTree(s.Pipeline, s.Manifests, s.Graph)
	if err != nil {
		return err
	}
	s.Tree = tree

	s.MapContexts = make(map[uuid.UUID]*MapContext)
	for mapID, members := range tree.Members {
		s.MapContexts[mapID] = &MapContext{
			MapBlockID:     mapID,
			MappedBlockIDs: members,
			ReduceBlockIDs: tree.Reduces[mapID],
			Expansions:     make(map[string][]ExpansionItem),
		}
	}
	return nil
}

// HandleMap processes the completion of one map block instance: it
// records the expansion for that instance and creates one invocation per
// member block per expansion item, each carrying the instance's index
// vector extended by the item index.  Instances that expand to zero
// items create no member invocations; the closing reduce for that
// instance becomes ready vacuously (see invocationReady).
func (s *SinglePipelineScheduler) HandleMap(result BlockInvocationResult) {
	invID := result.InvocationID()

	// The map block itself is complete — its "output" is the expansion.
	s.CompletedBlocks[invID] = result
	delete(s.PendingBlocks, invID)

	ctx, ok := s.MapContexts[result.Id]
	if !ok {
		return
	}

	var items []ExpansionItem
	if result.Expansion != nil {
		items = result.Expansion.Items
	}
	prefix := IndexPrefix(result.MapIndices, len(result.MapIndices))
	ctx.Expansions[prefix] = items

	blockByID := make(map[uuid.UUID]PipelineBlock, len(s.Pipeline.Blocks))
	for _, pb := range s.Pipeline.Blocks {
		blockByID[pb.Id] = pb
	}

	for blockID := range ctx.MappedBlockIDs {
		// Drop the placeholder pending entry (bare UUID): this block is
		// fanned out per instance from here on.
		delete(s.PendingBlocks, blockID.String())

		pb, found := blockByID[blockID]
		if !found {
			continue
		}
		for j := range items {
			indices := append(append([]int{}, result.MapIndices...), j)
			inv := BlockInvocation{
				Id:         pb.Id,
				PipelineId: s.Pipeline.Id,
				BlockId:    pb.Name,
				Inputs:     pb.Inputs,
				Arguments:  pb.Args,
				MapIndices: indices,
			}
			key := inv.InvocationID()
			// Idempotency: a replayed map result must not resurrect
			// invocations that already completed.
			if _, done := s.CompletedBlocks[key]; done {
				continue
			}
			s.PendingBlocks[key] = inv
		}
	}
}

// --- Multi-Tenant Scheduler ---

// MultiTenantScheduler manages multiple pipelines across multiple workers.
type MultiTenantScheduler struct {
	ExecutionQueue    []BlockInvocation
	Pipelines         map[uuid.UUID]Pipeline
	Schedulers        map[uuid.UUID]*SinglePipelineScheduler // pointer values for mutation
	Workers           map[uuid.UUID]Worker
	CurrentExecutions map[uuid.UUID]BlockInvocation
	roundRobinIndex   int
	pipelineOrder     []uuid.UUID // stable ordering for fair scheduling
}

func (s *MultiTenantScheduler) AddPipeline(p Pipeline) error {
	s.Pipelines[p.Id] = p
	scheduler := NewSchedulerForPipeline(p)
	s.Schedulers[p.Id] = scheduler
	s.pipelineOrder = append(s.pipelineOrder, p.Id)
	return nil
}

func (s *MultiTenantScheduler) CancelPipeline(id uuid.UUID) error {
	ps, ok := s.Schedulers[id]
	if !ok {
		return errors.New("failed to find pipeline")
	}
	ps.CancelPipeline(id)
	delete(s.Pipelines, id)
	// Remove from pipeline order
	for i, pid := range s.pipelineOrder {
		if pid == id {
			s.pipelineOrder = append(s.pipelineOrder[:i], s.pipelineOrder[i+1:]...)
			break
		}
	}
	return nil
}

func (s *MultiTenantScheduler) Update(invocationId uuid.UUID, result BlockInvocationResult) error {
	scheduler, ok := s.Schedulers[result.PipelineId]
	if !ok {
		return errors.New("could not find pipeline")
	}

	err := scheduler.Update(result)
	if err != nil {
		return err
	}

	// Remove from current executions
	delete(s.CurrentExecutions, invocationId)

	return nil
}

func (s *MultiTenantScheduler) AddWorker(worker Worker) error {
	s.Workers[worker.Id] = worker
	return nil
}

func (s *MultiTenantScheduler) RemoveWorker(id uuid.UUID) error {
	delete(s.Workers, id)
	return nil
}

func (s *MultiTenantScheduler) Next(workerId uuid.UUID) (BlockInvocation, bool, error) {
	// Return from execution queue first
	if len(s.ExecutionQueue) > 0 {
		block := s.ExecutionQueue[0]
		s.ExecutionQueue = s.ExecutionQueue[1:]
		s.CurrentExecutions[block.Id] = block
		return block, false, nil
	}

	// Fair round-robin scheduling across pipelines
	if len(s.pipelineOrder) == 0 {
		return BlockInvocation{}, true, nil
	}

	checked := 0
	for checked < len(s.pipelineOrder) {
		idx := s.roundRobinIndex % len(s.pipelineOrder)
		s.roundRobinIndex++
		checked++

		pipelineID := s.pipelineOrder[idx]
		scheduler, ok := s.Schedulers[pipelineID]
		if !ok {
			continue
		}

		if scheduler.IsReady() {
			invocation, done, err := scheduler.Next()
			if err != nil {
				fmt.Printf("error processing %s: %v\n", pipelineID, err)
				continue
			}
			if !done {
				s.CurrentExecutions[invocation.Id] = invocation
				return invocation, false, nil
			}
		}
	}

	// Check if all pipelines are done
	allDone := true
	for _, pid := range s.pipelineOrder {
		scheduler := s.Schedulers[pid]
		if scheduler != nil && !scheduler.Cancelled && (len(scheduler.PendingBlocks) > 0 || len(scheduler.ExecutableBlocks) > 0) {
			allDone = false
			break
		}
	}

	return BlockInvocation{}, allDone, nil
}

// NewSchedulerForPipeline creates and initializes a SinglePipelineScheduler.
func NewSchedulerForPipeline(pipeline Pipeline) *SinglePipelineScheduler {
	scheduler := &SinglePipelineScheduler{
		Cancelled:        false,
		ExecutableBlocks: []BlockInvocation{},
		CompletedBlocks:  map[string]BlockInvocationResult{},
		PendingBlocks:    map[string]BlockInvocation{},
		MapContexts:      map[uuid.UUID]*MapContext{},
	}
	scheduler.AddPipeline(pipeline)
	return scheduler
}
