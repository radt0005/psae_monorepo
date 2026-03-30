package core

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// MapContext tracks blocks within a map fan-out context.
type MapContext struct {
	MapBlockID     uuid.UUID
	ExpansionItems []ExpansionItem
	MappedBlockIDs map[uuid.UUID]bool // set of block UUIDs in this map context
	ReduceBlockID  uuid.UUID
}

// SinglePipelineScheduler schedules blocks for a single pipeline.
type SinglePipelineScheduler struct {
	Pipeline         Pipeline
	Cancelled        bool
	ExecutableBlocks []BlockInvocation
	CompletedBlocks  map[uuid.UUID]BlockInvocationResult
	PendingBlocks    map[uuid.UUID]BlockInvocation
	Manifests        map[string]BlockManifest
	MapContexts      map[uuid.UUID]*MapContext
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
			s.PendingBlocks[item.Id] = invocation
		}
	}

	return nil
}

func (s *SinglePipelineScheduler) CancelPipeline(id uuid.UUID) error {
	s.Cancelled = true
	s.PendingBlocks = map[uuid.UUID]BlockInvocation{}
	s.ExecutableBlocks = nil
	return nil
}

func (s *SinglePipelineScheduler) Update(result BlockInvocationResult) error {
	if result.Status == ExecutionStatusError {
		// Halt the pipeline: clear pending and executable, preserve completed for debugging
		s.PendingBlocks = map[uuid.UUID]BlockInvocation{}
		s.ExecutableBlocks = nil
		s.Cancelled = true
		return nil
	}

	if result.Status == ExecutionStatusComplete {
		s.CompletedBlocks[result.Id] = result

		// Find blocks that are now executable
		var newlyExecutable []uuid.UUID
		for id, pending := range s.PendingBlocks {
			allDepsCompleted := true
			for _, input := range pending.Inputs {
				var depID uuid.UUID
				if input.Block != nil {
					depID = *input.Block
				} else {
					depID = input.ID
				}
				if depID == uuid.Nil {
					continue
				}
				if _, completed := s.CompletedBlocks[depID]; !completed {
					allDepsCompleted = false
					break
				}
			}
			if allDepsCompleted {
				newlyExecutable = append(newlyExecutable, id)
			}
		}

		// Move newly executable blocks from pending to executable
		for _, id := range newlyExecutable {
			invocation := s.PendingBlocks[id]
			s.ExecutableBlocks = append(s.ExecutableBlocks, invocation)
			delete(s.PendingBlocks, id)
		}
	}

	if result.Status == ExecutionStatusMap {
		s.HandleMap(result)
	}

	if result.Status == ExecutionStatusReduce {
		s.HandleReduce(result)
	}

	return nil
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

// IdentifyMapContexts walks the dependency graph from each map block forward
// until a reduce block is reached. All blocks between are part of the map context.
func (s *SinglePipelineScheduler) IdentifyMapContexts() error {
	if s.Manifests == nil {
		return nil
	}
	s.MapContexts = make(map[uuid.UUID]*MapContext)

	// Build lookup from ID to pipeline block
	blockByID := make(map[uuid.UUID]PipelineBlock)
	for _, b := range s.Pipeline.Blocks {
		blockByID[b.Id] = b
	}

	for _, block := range s.Pipeline.Blocks {
		manifest, ok := s.Manifests[block.Name]
		if !ok || manifest.Kind != BlockKindMap {
			continue
		}

		ctx := &MapContext{
			MapBlockID:     block.Id,
			MappedBlockIDs: make(map[uuid.UUID]bool),
		}

		// Walk forward from map block to find all blocks in context until reduce
		s.walkMapContext(block.Id, ctx, blockByID)

		s.MapContexts[block.Id] = ctx
	}

	return nil
}

func (s *SinglePipelineScheduler) walkMapContext(id uuid.UUID, ctx *MapContext, blocks map[uuid.UUID]PipelineBlock) {
	for _, downstream := range s.Graph.Forward[id] {
		block, ok := blocks[downstream]
		if !ok {
			continue
		}
		manifest, ok := s.Manifests[block.Name]
		if !ok {
			continue
		}
		if manifest.Kind == BlockKindReduce {
			ctx.ReduceBlockID = downstream
			continue
		}
		ctx.MappedBlockIDs[downstream] = true
		s.walkMapContext(downstream, ctx, blocks)
	}
}

// HandleMap processes map block completion by creating N invocations for downstream blocks.
func (s *SinglePipelineScheduler) HandleMap(result BlockInvocationResult) {
	if result.Expansion == nil || len(result.Expansion.Items) == 0 {
		return
	}

	// Find the map context for this block
	ctx, ok := s.MapContexts[result.Id]
	if !ok {
		return
	}
	ctx.ExpansionItems = result.Expansion.Items

	n := len(result.Expansion.Items)

	// Mark the map block as completed
	s.CompletedBlocks[result.Id] = result

	// Create N invocations for each mapped block in the context
	for blockID := range ctx.MappedBlockIDs {
		original, ok := s.PendingBlocks[blockID]
		if !ok {
			// Try looking it up in the pipeline
			for _, pb := range s.Pipeline.Blocks {
				if pb.Id == blockID {
					original = BlockInvocation{
						Id:         pb.Id,
						PipelineId: s.Pipeline.Id,
						BlockId:    pb.Name,
						Inputs:     pb.Inputs,
						Arguments:  pb.Args,
					}
					break
				}
			}
		}
		// Remove the original pending block
		delete(s.PendingBlocks, blockID)

		for i := 0; i < n; i++ {
			idx := i
			inv := BlockInvocation{
				Id:         original.Id,
				PipelineId: original.PipelineId,
				BlockId:    original.BlockId,
				Inputs:     original.Inputs,
				Arguments:  original.Arguments,
				MapIndex:   &idx,
			}
			// Use a unique key for the mapped invocation in PendingBlocks
			mappedKey := uuid.NewSHA1(original.Id, []byte(fmt.Sprintf("%d", i)))

			// Check if this invocation's dependencies are all met
			allDepsCompleted := true
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
				if _, completed := s.CompletedBlocks[depID]; !completed {
					// Check if the dependency is the map block itself (already completed above)
					allDepsCompleted = false
					break
				}
			}

			if allDepsCompleted {
				s.ExecutableBlocks = append(s.ExecutableBlocks, inv)
			} else {
				s.PendingBlocks[mappedKey] = inv
			}
		}
	}
}

// HandleReduce detects when all N mapped invocations have completed and makes the
// reduce block executable.
func (s *SinglePipelineScheduler) HandleReduce(result BlockInvocationResult) {
	// Find which map context this result belongs to
	for _, ctx := range s.MapContexts {
		if ctx.ReduceBlockID == uuid.Nil {
			continue
		}

		// Check if all mapped invocations are complete
		allComplete := true
		n := len(ctx.ExpansionItems)
		if n == 0 {
			continue
		}

		for blockID := range ctx.MappedBlockIDs {
			for i := 0; i < n; i++ {
				mappedKey := uuid.NewSHA1(blockID, []byte(fmt.Sprintf("%d", i)))
				if _, ok := s.CompletedBlocks[mappedKey]; !ok {
					// Also check if it was directly completed by ID
					allComplete = false
					break
				}
			}
			if !allComplete {
				break
			}
		}

		if allComplete {
			// Create reduce invocation
			for _, pb := range s.Pipeline.Blocks {
				if pb.Id == ctx.ReduceBlockID {
					inv := BlockInvocation{
						Id:         pb.Id,
						PipelineId: s.Pipeline.Id,
						BlockId:    pb.Name,
						Inputs:     pb.Inputs,
						Arguments:  pb.Args,
					}
					s.ExecutableBlocks = append(s.ExecutableBlocks, inv)
					delete(s.PendingBlocks, pb.Id)
					break
				}
			}
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
		CompletedBlocks:  map[uuid.UUID]BlockInvocationResult{},
		PendingBlocks:    map[uuid.UUID]BlockInvocation{},
		MapContexts:      map[uuid.UUID]*MapContext{},
	}
	scheduler.AddPipeline(pipeline)
	return scheduler
}
