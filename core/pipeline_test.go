package core

import (
	"testing"

	"github.com/google/uuid"
)

// Helper to create a simple pipeline for testing
func makeLinearPipeline() Pipeline {
	idA, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	idB, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")
	idC, _ := uuid.Parse("019cf4bc-3333-7000-0000-000000000000")

	return Pipeline{
		Id:      uuid.New(),
		Name:    "test",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idA, Name: "block.a", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: idB, Name: "block.b", Inputs: []InputRef{{ID: idA}}, Args: map[string]any{}},
			{Id: idC, Name: "block.c", Inputs: []InputRef{{ID: idB}}, Args: map[string]any{}},
		},
	}
}

func TestBuildDependencyGraphLinear(t *testing.T) {
	p := makeLinearPipeline()
	g, err := BuildDependencyGraph(p)
	if err != nil {
		t.Fatalf("BuildDependencyGraph failed: %v", err)
	}

	idA := p.Blocks[0].Id
	idB := p.Blocks[1].Id
	idC := p.Blocks[2].Id

	// A -> B edge
	if len(g.Forward[idA]) != 1 || g.Forward[idA][0] != idB {
		t.Errorf("expected A -> B edge, got %v", g.Forward[idA])
	}
	// B -> C edge
	if len(g.Forward[idB]) != 1 || g.Forward[idB][0] != idC {
		t.Errorf("expected B -> C edge, got %v", g.Forward[idB])
	}
	// C has no forward edges
	if len(g.Forward[idC]) != 0 {
		t.Errorf("expected C to have no forward edges, got %v", g.Forward[idC])
	}
}

func TestTopologicalSortDiamond(t *testing.T) {
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

	g, err := BuildDependencyGraph(p)
	if err != nil {
		t.Fatalf("BuildDependencyGraph failed: %v", err)
	}

	sorted, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	if len(sorted) != 4 {
		t.Fatalf("expected 4 sorted nodes, got %d", len(sorted))
	}

	// A must come before B, C; B and C must come before D
	indexOf := make(map[uuid.UUID]int)
	for i, id := range sorted {
		indexOf[id] = i
	}

	if indexOf[idA] > indexOf[idB] {
		t.Error("A must come before B")
	}
	if indexOf[idA] > indexOf[idC] {
		t.Error("A must come before C")
	}
	if indexOf[idB] > indexOf[idD] {
		t.Error("B must come before D")
	}
	if indexOf[idC] > indexOf[idD] {
		t.Error("C must come before D")
	}
}

func TestTopologicalSortCycleDetection(t *testing.T) {
	idA, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	idB, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")
	idC, _ := uuid.Parse("019cf4bc-3333-7000-0000-000000000000")

	p := Pipeline{
		Id:      uuid.New(),
		Name:    "cycle",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idA, Name: "block.a", Inputs: []InputRef{{ID: idC}}, Args: map[string]any{}},
			{Id: idB, Name: "block.b", Inputs: []InputRef{{ID: idA}}, Args: map[string]any{}},
			{Id: idC, Name: "block.c", Inputs: []InputRef{{ID: idB}}, Args: map[string]any{}},
		},
	}

	g, err := BuildDependencyGraph(p)
	if err != nil {
		t.Fatalf("BuildDependencyGraph failed: %v", err)
	}

	_, err = g.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}
}

func TestSourceBlocks(t *testing.T) {
	p := makeLinearPipeline()
	g, _ := BuildDependencyGraph(p)

	sources := g.SourceBlocks()
	if len(sources) != 1 {
		t.Fatalf("expected 1 source block, got %d", len(sources))
	}
	if sources[0] != p.Blocks[0].Id {
		t.Errorf("expected source block %s, got %s", p.Blocks[0].Id, sources[0])
	}
}

func TestResolveInputsBareReference(t *testing.T) {
	depID, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	blockID, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")

	block := PipelineBlock{
		Id:     blockID,
		Name:   "block.b",
		Inputs: []InputRef{{ID: depID}},
		Args:   map[string]any{},
	}

	depManifest := BlockManifest{
		ID: "block.a",
		Outputs: map[string]OutputDeclaration{
			"raster": {Type: "file", Format: "GeoTIFF"},
		},
	}

	currentManifest := BlockManifest{
		ID: "block.b",
		Inputs: map[string]InputDeclaration{
			"source": {Type: "file", Format: "GeoTIFF"},
		},
	}

	deps := map[uuid.UUID]BlockManifest{depID: depManifest}
	resolved, err := ResolveInputs(block, deps, currentManifest)
	if err != nil {
		t.Fatalf("ResolveInputs failed: %v", err)
	}

	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved input, got %d", len(resolved))
	}

	ri, ok := resolved["source"]
	if !ok {
		t.Fatal("expected resolved input 'source'")
	}
	if ri.SourceBlockID != depID {
		t.Errorf("expected source block %s, got %s", depID, ri.SourceBlockID)
	}
	if ri.SourceOutputName != "raster" {
		t.Errorf("expected source output 'raster', got %q", ri.SourceOutputName)
	}
}

func TestResolveInputsExplicitReference(t *testing.T) {
	depID, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	blockID, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")

	block := PipelineBlock{
		Id:     blockID,
		Name:   "block.b",
		Inputs: []InputRef{{Block: &depID, Output: "raster"}},
		Args:   map[string]any{},
	}

	depManifest := BlockManifest{
		ID: "block.a",
		Outputs: map[string]OutputDeclaration{
			"raster":  {Type: "file", Format: "GeoTIFF"},
			"summary": {Type: "json"},
		},
	}

	currentManifest := BlockManifest{
		ID: "block.b",
		Inputs: map[string]InputDeclaration{
			"source": {Type: "file", Format: "GeoTIFF"},
		},
	}

	deps := map[uuid.UUID]BlockManifest{depID: depManifest}
	resolved, err := ResolveInputs(block, deps, currentManifest)
	if err != nil {
		t.Fatalf("ResolveInputs failed: %v", err)
	}

	ri, ok := resolved["source"]
	if !ok {
		t.Fatal("expected resolved input 'source'")
	}
	if ri.SourceOutputName != "raster" {
		t.Errorf("expected source output 'raster', got %q", ri.SourceOutputName)
	}
}

func TestResolveInputsAmbiguous(t *testing.T) {
	depID, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	blockID, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")

	block := PipelineBlock{
		Id:     blockID,
		Name:   "block.b",
		Inputs: []InputRef{{ID: depID}},
		Args:   map[string]any{},
	}

	depManifest := BlockManifest{
		ID: "block.a",
		Outputs: map[string]OutputDeclaration{
			"raster1": {Type: "file"},
			"raster2": {Type: "file"},
		},
	}

	currentManifest := BlockManifest{
		ID: "block.b",
		Inputs: map[string]InputDeclaration{
			"source1": {Type: "file"},
			"source2": {Type: "file"},
		},
	}

	deps := map[uuid.UUID]BlockManifest{depID: depManifest}
	_, err := ResolveInputs(block, deps, currentManifest)
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
}

func TestResolveInputsIncomplete(t *testing.T) {
	depID, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	blockID, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")

	block := PipelineBlock{
		Id:     blockID,
		Name:   "block.b",
		Inputs: []InputRef{{ID: depID}},
		Args:   map[string]any{},
	}

	depManifest := BlockManifest{
		ID: "block.a",
		Outputs: map[string]OutputDeclaration{
			"raster": {Type: "file"},
		},
	}

	currentManifest := BlockManifest{
		ID: "block.b",
		Inputs: map[string]InputDeclaration{
			"source":    {Type: "file"},
			"boundary":  {Type: "directory"}, // no match
		},
	}

	deps := map[uuid.UUID]BlockManifest{depID: depManifest}
	_, err := ResolveInputs(block, deps, currentManifest)
	if err == nil {
		t.Fatal("expected incomplete error, got nil")
	}
}

func TestValidatePipelineDuplicateIDs(t *testing.T) {
	id, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	p := Pipeline{
		Id:      uuid.New(),
		Name:    "dup",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: id, Name: "block.a", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: id, Name: "block.b", Inputs: []InputRef{}, Args: map[string]any{}},
		},
	}

	manifests := map[string]BlockManifest{
		"block.a": {ID: "block.a", Inputs: map[string]InputDeclaration{}, Outputs: map[string]OutputDeclaration{}},
		"block.b": {ID: "block.b", Inputs: map[string]InputDeclaration{}, Outputs: map[string]OutputDeclaration{}},
	}

	errs := ValidatePipeline(p, manifests)
	if len(errs) == 0 {
		t.Fatal("expected validation errors for duplicate IDs")
	}

	found := false
	for _, e := range errs {
		if e.Error() == "duplicate block id: "+id.String() {
			found = true
		}
	}
	if !found {
		t.Error("expected duplicate block id error")
	}
}

func TestValidatePipelineNonExistentRef(t *testing.T) {
	idA, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	idB, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")
	nonExist, _ := uuid.Parse("019cf4bc-9999-7000-0000-000000000000")

	p := Pipeline{
		Id:      uuid.New(),
		Name:    "bad-ref",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idA, Name: "block.a", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: idB, Name: "block.b", Inputs: []InputRef{{ID: nonExist}}, Args: map[string]any{}},
		},
	}

	manifests := map[string]BlockManifest{
		"block.a": {ID: "block.a", Inputs: map[string]InputDeclaration{}, Outputs: map[string]OutputDeclaration{}},
		"block.b": {ID: "block.b", Inputs: map[string]InputDeclaration{}, Outputs: map[string]OutputDeclaration{}},
	}

	errs := ValidatePipeline(p, manifests)
	if len(errs) == 0 {
		t.Fatal("expected validation errors for non-existent reference")
	}
}

func TestValidatePipelineUnknownBlockType(t *testing.T) {
	idA, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")

	p := Pipeline{
		Id:      uuid.New(),
		Name:    "unknown-type",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idA, Name: "block.unknown", Inputs: []InputRef{}, Args: map[string]any{}},
		},
	}

	manifests := map[string]BlockManifest{} // empty

	errs := ValidatePipeline(p, manifests)
	found := false
	for _, e := range errs {
		if e != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("expected validation error for unknown block type")
	}
}

func TestValidateMapWithoutReduce(t *testing.T) {
	idA, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	idB, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")

	p := Pipeline{
		Id:      uuid.New(),
		Name:    "map-no-reduce",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idA, Name: "core.map.files", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: idB, Name: "raster.process", Inputs: []InputRef{{ID: idA}}, Args: map[string]any{}},
		},
	}

	manifests := map[string]BlockManifest{
		"core.map.files": {
			ID: "core.map.files", Kind: BlockKindMap,
			Inputs:  map[string]InputDeclaration{"source": {Type: "collection"}},
			Outputs: map[string]OutputDeclaration{"manifest": {Type: "expansion"}},
		},
		"raster.process": {
			ID: "raster.process", Kind: BlockKindStandard,
			Inputs:  map[string]InputDeclaration{"data": {Type: "file"}},
			Outputs: map[string]OutputDeclaration{"result": {Type: "file"}},
		},
	}

	errs := ValidatePipeline(p, manifests)
	found := false
	for _, e := range errs {
		if e != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("expected validation error for map without reduce")
	}
}

func TestValidateNestedMaps(t *testing.T) {
	idA, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	idB, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")
	idC, _ := uuid.Parse("019cf4bc-3333-7000-0000-000000000000")

	p := Pipeline{
		Id:      uuid.New(),
		Name:    "nested-map",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idA, Name: "core.map.files", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: idB, Name: "core.map.files2", Inputs: []InputRef{{ID: idA}}, Args: map[string]any{}},
			{Id: idC, Name: "core.reduce", Inputs: []InputRef{{ID: idB}}, Args: map[string]any{}},
		},
	}

	manifests := map[string]BlockManifest{
		"core.map.files": {
			ID: "core.map.files", Kind: BlockKindMap,
			Inputs:  map[string]InputDeclaration{"source": {Type: "collection"}},
			Outputs: map[string]OutputDeclaration{"manifest": {Type: "expansion"}},
		},
		"core.map.files2": {
			ID: "core.map.files2", Kind: BlockKindMap,
			Inputs:  map[string]InputDeclaration{"source": {Type: "collection"}},
			Outputs: map[string]OutputDeclaration{"manifest": {Type: "expansion"}},
		},
		"core.reduce": {
			ID: "core.reduce", Kind: BlockKindReduce,
			Inputs:  map[string]InputDeclaration{"tiles": {Type: "collection"}},
			Outputs: map[string]OutputDeclaration{"result": {Type: "file"}},
		},
	}

	errs := ValidatePipeline(p, manifests)
	found := false
	for _, e := range errs {
		if e != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("expected validation error for nested maps")
	}
}

func TestValidateMapBlockMustOutputExpansion(t *testing.T) {
	idA, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	idB, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")

	p := Pipeline{
		Id:      uuid.New(),
		Name:    "map-no-expansion",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: idA, Name: "bad.map", Inputs: []InputRef{}, Args: map[string]any{}},
			{Id: idB, Name: "core.reduce", Inputs: []InputRef{{ID: idA}}, Args: map[string]any{}},
		},
	}

	manifests := map[string]BlockManifest{
		"bad.map": {
			ID: "bad.map", Kind: BlockKindMap,
			Inputs:  map[string]InputDeclaration{"source": {Type: "collection"}},
			Outputs: map[string]OutputDeclaration{"result": {Type: "file"}}, // should be expansion!
		},
		"core.reduce": {
			ID: "core.reduce", Kind: BlockKindReduce,
			Inputs:  map[string]InputDeclaration{"tiles": {Type: "collection"}},
			Outputs: map[string]OutputDeclaration{"result": {Type: "file"}},
		},
	}

	errs := ValidatePipeline(p, manifests)
	found := false
	for _, e := range errs {
		if e != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("expected validation error for map block without expansion output")
	}
}
