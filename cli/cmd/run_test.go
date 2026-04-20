package cmd

import (
	"core"
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
