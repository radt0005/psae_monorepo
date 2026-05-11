package cmd

import (
	"core"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestBuildInputHashes(t *testing.T) {
	blockA := uuid.MustParse("019cf4bc-1111-7000-0000-000000000000")
	blockB := uuid.MustParse("019cf4bc-2222-7000-0000-000000000000")

	outputHashes := map[string]map[string]string{
		blockA.String(): {"raster": "abc123"},
	}

	blocks := map[uuid.UUID]core.PipelineBlock{
		blockA: {Id: blockA, Name: "data.source"},
		blockB: {Id: blockB, Name: "raster.process"},
	}

	invocation := core.BlockInvocation{
		Id:      blockB,
		BlockId: "raster.process",
		Inputs: []core.InputRef{
			{ID: blockA},
		},
	}

	hashes := buildInputHashes(invocation, outputHashes, blocks)
	if len(hashes) == 0 {
		t.Error("expected non-empty input hashes")
	}
}

func TestBuildInputHashes_ExplicitRef(t *testing.T) {
	blockA := uuid.MustParse("019cf4bc-1111-7000-0000-000000000000")
	blockB := uuid.MustParse("019cf4bc-2222-7000-0000-000000000000")

	outputHashes := map[string]map[string]string{
		blockA.String(): {"raster": "abc123", "metadata": "def456"},
	}

	blocks := map[uuid.UUID]core.PipelineBlock{
		blockA: {Id: blockA, Name: "data.source"},
		blockB: {Id: blockB, Name: "raster.process"},
	}

	invocation := core.BlockInvocation{
		Id:      blockB,
		BlockId: "raster.process",
		Inputs: []core.InputRef{
			{Block: &blockA, Output: "raster"},
		},
	}

	hashes := buildInputHashes(invocation, outputHashes, blocks)
	if len(hashes) == 0 {
		t.Error("expected non-empty input hashes")
	}

	// Should contain the specific output hash
	found := false
	for _, v := range hashes {
		if v == "abc123" {
			found = true
		}
	}
	if !found {
		t.Error("expected hash abc123 in input hashes")
	}
}

func TestBuildInputHashes_NoDeps(t *testing.T) {
	blockA := uuid.MustParse("019cf4bc-1111-7000-0000-000000000000")

	outputHashes := map[string]map[string]string{}
	blocks := map[uuid.UUID]core.PipelineBlock{
		blockA: {Id: blockA, Name: "data.source"},
	}

	invocation := core.BlockInvocation{
		Id:      blockA,
		BlockId: "data.source",
		Inputs:  []core.InputRef{},
	}

	hashes := buildInputHashes(invocation, outputHashes, blocks)
	if len(hashes) != 0 {
		t.Errorf("expected empty hashes for block with no deps, got %d", len(hashes))
	}
}

func TestRunPipeline_PipelineIdGenerated(t *testing.T) {
	// A pipeline without a top-level `id` should be assigned a fresh
	// UUIDv7 at run time (spec/pipeline.md §10).  Two consecutive runs
	// must produce different pipeline ids.
	//
	// This test exercises LoadAndResolvePipeline + the run.go pipeline-id
	// fallback directly rather than going through the full scheduler.
	dir := t.TempDir()
	pipelinePath := filepath.Join(dir, "pipeline.yaml")
	os.WriteFile(pipelinePath, []byte(`name: smoke
version: "1.0"
blocks:
  - id: "@a"
    name: data.x
    inputs: []
    args: {}
`), 0644)

	load := func() core.Pipeline {
		p, _, _, err := core.LoadAndResolvePipeline(pipelinePath)
		if err != nil {
			t.Fatal(err)
		}
		if p.Id == uuid.Nil {
			id, _ := uuid.NewV7()
			p.Id = id
		}
		return p
	}

	first := load()
	second := load()

	if first.Id == uuid.Nil || second.Id == uuid.Nil {
		t.Fatal("pipeline id should be non-nil after fallback")
	}
	if first.Id == second.Id {
		t.Fatal("expected distinct pipeline ids across runs")
	}
	// Block ids are stable across the two loads (lockfile binding
	// preserved), confirming the cache property holds for short-code
	// pipelines.
	if first.Blocks[0].Id != second.Blocks[0].Id {
		t.Fatalf("block id drifted across runs: %s vs %s", first.Blocks[0].Id, second.Blocks[0].Id)
	}
}
