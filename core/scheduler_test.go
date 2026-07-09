package core

import (
	"testing"

	"github.com/google/uuid"
)

func TestSinglePipelineSchedulerLinear(t *testing.T) {
	idA, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	idB, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")
	idC, _ := uuid.Parse("019cf4bc-3333-7000-0000-000000000000")

	p := Pipeline{
		Id:      uuid.New(),
		Name:    "linear",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idA, Name: "block.a", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: idB, Name: "block.b", Inputs: []InputRef{{ID: idA}}, Args: map[string]any{}},
			{Id: idC, Name: "block.c", Inputs: []InputRef{{ID: idB}}, Args: map[string]any{}},
		},
	}

	s := NewSchedulerForPipeline(p)

	// A should be immediately executable (source block)
	if !s.IsReady() {
		t.Fatal("scheduler should be ready with source block A")
	}

	inv, done, err := s.Next()
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if done {
		t.Fatal("should not be done yet")
	}
	if inv.Id != idA {
		t.Errorf("expected block A, got %s", inv.Id)
	}

	// After A completes, B should become executable
	s.Update(BlockInvocationResult{Id: idA, PipelineId: p.Id, Status: ExecutionStatusComplete})
	if !s.IsReady() {
		t.Fatal("scheduler should be ready with block B")
	}

	inv, _, _ = s.Next()
	if inv.Id != idB {
		t.Errorf("expected block B, got %s", inv.Id)
	}

	// After B completes, C should become executable
	s.Update(BlockInvocationResult{Id: idB, PipelineId: p.Id, Status: ExecutionStatusComplete})
	if !s.IsReady() {
		t.Fatal("scheduler should be ready with block C")
	}

	inv, _, _ = s.Next()
	if inv.Id != idC {
		t.Errorf("expected block C, got %s", inv.Id)
	}

	// After C completes, we should be done
	s.Update(BlockInvocationResult{Id: idC, PipelineId: p.Id, Status: ExecutionStatusComplete})
	_, done, _ = s.Next()
	if !done {
		t.Fatal("should be done after all blocks complete")
	}
}

func TestSinglePipelineSchedulerParallel(t *testing.T) {
	idA, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	idB, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")
	idC, _ := uuid.Parse("019cf4bc-3333-7000-0000-000000000000")

	p := Pipeline{
		Id:      uuid.New(),
		Name:    "parallel",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idA, Name: "block.a", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: idB, Name: "block.b", Inputs: []InputRef{{ID: idA}}, Args: map[string]any{}},
			{Id: idC, Name: "block.c", Inputs: []InputRef{{ID: idA}}, Args: map[string]any{}},
		},
	}

	s := NewSchedulerForPipeline(p)

	// Execute A
	inv, _, _ := s.Next()
	if inv.Id != idA {
		t.Fatalf("expected A first, got %s", inv.Id)
	}

	// Complete A - both B and C should become executable
	s.Update(BlockInvocationResult{Id: idA, PipelineId: p.Id, Status: ExecutionStatusComplete})

	if len(s.ExecutableBlocks) != 2 {
		t.Fatalf("expected 2 executable blocks after A completes, got %d", len(s.ExecutableBlocks))
	}
}

func TestSinglePipelineSchedulerDiamond(t *testing.T) {
	idA, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	idB, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")
	idC, _ := uuid.Parse("019cf4bc-3333-7000-0000-000000000000")
	idD, _ := uuid.Parse("019cf4bc-4444-7000-0000-000000000000")

	p := Pipeline{
		Id:      uuid.New(),
		Name:    "diamond",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idA, Name: "block.a", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: idB, Name: "block.b", Inputs: []InputRef{{ID: idA}}, Args: map[string]any{}},
			{Id: idC, Name: "block.c", Inputs: []InputRef{{ID: idA}}, Args: map[string]any{}},
			{Id: idD, Name: "block.d", Inputs: []InputRef{{ID: idB}, {ID: idC}}, Args: map[string]any{}},
		},
	}

	s := NewSchedulerForPipeline(p)

	// Execute A
	inv, _, _ := s.Next()
	s.Update(BlockInvocationResult{Id: inv.Id, PipelineId: p.Id, Status: ExecutionStatusComplete})

	// B and C should both be executable
	if len(s.ExecutableBlocks) != 2 {
		t.Fatalf("expected 2 executable blocks, got %d", len(s.ExecutableBlocks))
	}

	// Complete B only - D should NOT be executable yet
	invB, _, _ := s.Next()
	s.Update(BlockInvocationResult{Id: invB.Id, PipelineId: p.Id, Status: ExecutionStatusComplete})

	// D should still be pending (waiting for C)
	if _, exists := s.PendingBlocks[idD.String()]; !exists {
		// Check if D is in executable (which would be wrong unless C also completed)
		for _, eb := range s.ExecutableBlocks {
			if eb.Id == idD {
				t.Fatal("D should not be executable until both B and C complete")
			}
		}
	}

	// Complete C - now D should become executable
	invC, _, _ := s.Next()
	s.Update(BlockInvocationResult{Id: invC.Id, PipelineId: p.Id, Status: ExecutionStatusComplete})

	if !s.IsReady() {
		t.Fatal("D should be executable after both B and C complete")
	}

	invD, _, _ := s.Next()
	if invD.Id != idD {
		t.Errorf("expected block D, got %s", invD.Id)
	}
}

func TestSinglePipelineSchedulerErrorHalts(t *testing.T) {
	idA, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	idB, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")

	p := Pipeline{
		Id:      uuid.New(),
		Name:    "error-test",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idA, Name: "block.a", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: idB, Name: "block.b", Inputs: []InputRef{{ID: idA}}, Args: map[string]any{}},
		},
	}

	s := NewSchedulerForPipeline(p)
	s.Next() // get A

	// Report A as error
	s.Update(BlockInvocationResult{Id: idA, PipelineId: p.Id, Status: ExecutionStatusError, Error: "something failed"})

	if !s.Cancelled {
		t.Fatal("pipeline should be cancelled after error")
	}
	if len(s.PendingBlocks) != 0 {
		t.Error("pending blocks should be cleared after error")
	}
	if len(s.ExecutableBlocks) != 0 {
		t.Error("executable blocks should be cleared after error")
	}
}

func TestHandleMapCreatesInvocations(t *testing.T) {
	idMap, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	idProcess, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")
	idReduce, _ := uuid.Parse("019cf4bc-3333-7000-0000-000000000000")

	p := Pipeline{
		Id:      uuid.New(),
		Name:    "map-test",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idMap, Name: "core.map.files", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: idProcess, Name: "raster.process", Inputs: []InputRef{{ID: idMap}}, Args: map[string]any{}},
			{Id: idReduce, Name: "core.reduce", Inputs: []InputRef{{ID: idProcess}}, Args: map[string]any{}},
		},
	}

	manifests := map[string]BlockManifest{
		"core.map.files": {ID: "core.map.files", Kind: BlockKindMap,
			Inputs:  map[string]InputDeclaration{"source": {Type: "collection"}},
			Outputs: map[string]OutputDeclaration{"manifest": {Type: "expansion"}}},
		"raster.process": {ID: "raster.process", Kind: BlockKindStandard,
			Inputs:  map[string]InputDeclaration{"data": {Type: "file"}},
			Outputs: map[string]OutputDeclaration{"result": {Type: "file"}}},
		"core.reduce": {ID: "core.reduce", Kind: BlockKindReduce,
			Inputs:  map[string]InputDeclaration{"tiles": {Type: "collection"}},
			Outputs: map[string]OutputDeclaration{"result": {Type: "file"}}},
	}

	s := NewSchedulerForPipeline(p)
	s.Manifests = manifests
	s.IdentifyMapContexts()

	// Execute map block
	inv, _, _ := s.Next()
	if inv.Id != idMap {
		t.Fatalf("expected map block first, got %s", inv.Id)
	}

	// Map block completes with 3 items
	expansion := &ExpansionManifest{
		Items: []ExpansionItem{
			{Path: "tile_001.tif", Key: "tile_001"},
			{Path: "tile_002.tif", Key: "tile_002"},
			{Path: "tile_003.tif", Key: "tile_003"},
		},
	}
	s.HandleMap(BlockInvocationResult{
		Id:        idMap,
		PipelineId: p.Id,
		Status:    ExecutionStatusMap,
		Expansion: expansion,
	})

	// Should have created 3 invocations for raster.process
	// They could be in ExecutableBlocks or PendingBlocks
	totalMapped := len(s.ExecutableBlocks)
	for _, pending := range s.PendingBlocks {
		if len(pending.MapIndices) > 0 {
			totalMapped++
		}
	}

	if totalMapped < 3 {
		t.Errorf("expected at least 3 mapped invocations, got %d (executable: %d, pending mapped: %d)",
			totalMapped, len(s.ExecutableBlocks), totalMapped-len(s.ExecutableBlocks))
	}

	// Check that invocations have MapIndices set
	for _, eb := range s.ExecutableBlocks {
		if len(eb.MapIndices) == 0 {
			t.Error("executable mapped block should have MapIndices set")
		}
	}
}

// TestReduceDirectlyFollowsMap covers the degenerate map context with no
// intermediate mapped block: map → reduce.  The reduce gathers the map's
// expansion items directly, so it must become executable as soon as the map
// completes.  Regression test for the fan-out stall where promoteReady was
// not run after a map result.
func TestReduceDirectlyFollowsMap(t *testing.T) {
	idMap, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	idReduce, _ := uuid.Parse("019cf4bc-3333-7000-0000-000000000000")

	p := Pipeline{
		Id:      uuid.New(),
		Name:    "map-reduce-direct",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idMap, Name: "base.map_list", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: idReduce, Name: "base.reduce_collection", Inputs: []InputRef{{ID: idMap}}, Args: map[string]any{}},
		},
	}
	manifests := map[string]BlockManifest{
		"base.map_list": {ID: "base.map_list", Kind: BlockKindMap,
			Outputs: map[string]OutputDeclaration{"manifest": {Type: "expansion"}}},
		"base.reduce_collection": {ID: "base.reduce_collection", Kind: BlockKindReduce,
			Inputs:  map[string]InputDeclaration{"items": {Type: "collection"}},
			Outputs: map[string]OutputDeclaration{"result": {Type: "file"}}},
	}

	s := NewSchedulerForPipeline(p)
	s.Manifests = manifests
	s.IdentifyMapContexts()

	// Map block runs first.
	inv, _, _ := s.Next()
	if inv.Id != idMap {
		t.Fatalf("expected map block first, got %s", inv.Id)
	}

	// Map completes with a 3-item expansion and no intermediate block.
	s.Update(BlockInvocationResult{
		Id:        idMap,
		PipelineId: p.Id,
		Status:    ExecutionStatusMap,
		Expansion: &ExpansionManifest{Items: []ExpansionItem{
			{Path: "00.json", Key: "NY"},
			{Path: "01.json", Key: "CA"},
			{Path: "02.json", Key: "MI"},
		}},
	})

	// The reduce must now be executable (not stuck pending).
	found := false
	for _, eb := range s.ExecutableBlocks {
		if eb.Id == idReduce {
			found = true
		}
	}
	if !found {
		t.Fatalf("reduce block was not promoted to executable after map completed (executable=%d, pending=%d)",
			len(s.ExecutableBlocks), len(s.PendingBlocks))
	}
}

func TestMultiTenantSchedulerTwoPipelines(t *testing.T) {
	p1 := Pipeline{
		Id:      uuid.New(),
		Name:    "pipeline-1",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: uuid.New(), Name: "block.a", Inputs: []InputRef{}, Args: map[string]any{}},
		},
	}
	p2 := Pipeline{
		Id:      uuid.New(),
		Name:    "pipeline-2",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: uuid.New(), Name: "block.b", Inputs: []InputRef{}, Args: map[string]any{}},
		},
	}

	mts := &MultiTenantScheduler{
		ExecutionQueue:    []BlockInvocation{},
		Pipelines:         map[uuid.UUID]Pipeline{},
		Schedulers:        map[uuid.UUID]*SinglePipelineScheduler{},
		Workers:           map[uuid.UUID]Worker{},
		CurrentExecutions: map[uuid.UUID]BlockInvocation{},
	}

	mts.AddPipeline(p1)
	mts.AddPipeline(p2)

	workerID := uuid.New()
	mts.AddWorker(Worker{Id: workerID})

	// Both pipelines should get work
	inv1, done, err := mts.Next(workerID)
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if done {
		t.Fatal("should not be done")
	}

	inv2, done, err := mts.Next(workerID)
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if done {
		t.Fatal("should not be done")
	}

	// Both pipelines should have gotten work (different pipeline IDs)
	if inv1.PipelineId == inv2.PipelineId {
		t.Error("expected work from different pipelines")
	}
}

func TestMultiTenantSchedulerErrorIsolation(t *testing.T) {
	p1 := Pipeline{
		Id:      uuid.New(),
		Name:    "fail-pipeline",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: uuid.New(), Name: "block.a", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: uuid.New(), Name: "block.b", Inputs: []InputRef{{ID: uuid.Nil}}, Args: map[string]any{}},
		},
	}
	// Fix p1's second block to reference the first block
	p1.Blocks[1].Inputs = []InputRef{{ID: p1.Blocks[0].Id}}

	p2 := Pipeline{
		Id:      uuid.New(),
		Name:    "ok-pipeline",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: uuid.New(), Name: "block.c", Inputs: []InputRef{}, Args: map[string]any{}},
		},
	}

	mts := &MultiTenantScheduler{
		ExecutionQueue:    []BlockInvocation{},
		Pipelines:         map[uuid.UUID]Pipeline{},
		Schedulers:        map[uuid.UUID]*SinglePipelineScheduler{},
		Workers:           map[uuid.UUID]Worker{},
		CurrentExecutions: map[uuid.UUID]BlockInvocation{},
	}

	mts.AddPipeline(p1)
	mts.AddPipeline(p2)

	workerID := uuid.New()

	// Get first block from p1
	inv, _, _ := mts.Next(workerID)

	// Report error for p1's block
	mts.Update(inv.Id, BlockInvocationResult{
		Id:         inv.Id,
		PipelineId: inv.PipelineId,
		Status:     ExecutionStatusError,
		Error:      "failed",
	})

	// p1's scheduler should be cancelled
	if !mts.Schedulers[p1.Id].Cancelled {
		t.Error("p1 should be cancelled after error")
	}

	// p2 should still work
	if mts.Schedulers[p2.Id].Cancelled {
		t.Error("p2 should NOT be cancelled")
	}
}

func TestMapContextPropagation(t *testing.T) {
	idMap, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	idB, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")
	idC, _ := uuid.Parse("019cf4bc-3333-7000-0000-000000000000")
	idReduce, _ := uuid.Parse("019cf4bc-4444-7000-0000-000000000000")

	p := Pipeline{
		Id:      uuid.New(),
		Name:    "chain-map",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idMap, Name: "core.map.files", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: idB, Name: "raster.ndvi", Inputs: []InputRef{{ID: idMap}}, Args: map[string]any{}},
			{Id: idC, Name: "raster.classify", Inputs: []InputRef{{ID: idB}}, Args: map[string]any{}},
			{Id: idReduce, Name: "core.reduce", Inputs: []InputRef{{ID: idC}}, Args: map[string]any{}},
		},
	}

	manifests := map[string]BlockManifest{
		"core.map.files":   {ID: "core.map.files", Kind: BlockKindMap, Inputs: map[string]InputDeclaration{}, Outputs: map[string]OutputDeclaration{"manifest": {Type: "expansion"}}},
		"raster.ndvi":      {ID: "raster.ndvi", Kind: BlockKindStandard, Inputs: map[string]InputDeclaration{}, Outputs: map[string]OutputDeclaration{}},
		"raster.classify":  {ID: "raster.classify", Kind: BlockKindStandard, Inputs: map[string]InputDeclaration{}, Outputs: map[string]OutputDeclaration{}},
		"core.reduce":      {ID: "core.reduce", Kind: BlockKindReduce, Inputs: map[string]InputDeclaration{"tiles": {Type: "collection"}}, Outputs: map[string]OutputDeclaration{}},
	}

	s := NewSchedulerForPipeline(p)
	s.Manifests = manifests
	s.IdentifyMapContexts()

	// The map context should include B and C (between map and reduce)
	ctx, ok := s.MapContexts[idMap]
	if !ok {
		t.Fatal("expected map context for map block")
	}

	if !ctx.MappedBlockIDs[idB] {
		t.Error("expected block B in map context")
	}
	if !ctx.MappedBlockIDs[idC] {
		t.Error("expected block C in map context")
	}
	if len(ctx.ReduceBlockIDs) != 1 || ctx.ReduceBlockIDs[0] != idReduce {
		t.Errorf("expected reduce blocks [%s], got %v", idReduce, ctx.ReduceBlockIDs)
	}
}
