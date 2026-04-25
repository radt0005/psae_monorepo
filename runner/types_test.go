package spade_runner

import (
	"core"
	"testing"

	"github.com/google/uuid"
)

func TestParseInvocationID_PlainUUID(t *testing.T) {
	u := uuid.New()
	got, idx, err := ParseInvocationID(u.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != u {
		t.Errorf("uuid mismatch: want %s got %s", u, got)
	}
	if idx != nil {
		t.Errorf("expected nil map index, got %v", *idx)
	}
}

func TestParseInvocationID_Mapped(t *testing.T) {
	u := uuid.New()
	got, idx, err := ParseInvocationID(u.String() + ".7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != u {
		t.Errorf("uuid mismatch")
	}
	if idx == nil || *idx != 7 {
		t.Fatalf("expected map index 7, got %v", idx)
	}
}

func TestParseInvocationID_Invalid(t *testing.T) {
	_, _, err := ParseInvocationID("not-a-uuid")
	if err == nil {
		t.Fatal("expected error for invalid UUID")
	}
}

func TestParseInvocationID_NegativeIndexTreatedAsPartOfUUID(t *testing.T) {
	// A negative "index" should not be parsed as a map suffix — it's not
	// a valid index and should fail UUID parsing instead.
	u := uuid.New()
	_, _, err := ParseInvocationID(u.String() + ".-1")
	if err == nil {
		t.Fatal("expected error: negative index is not a valid invocation suffix")
	}
}

func TestInvocationFromJob_Standard(t *testing.T) {
	blockID := uuid.New()
	pipeID := uuid.New()
	j := Job{
		Assignment: core.WorkerAssignment{
			InvocationID: blockID.String(),
			BlockName:    "hello",
			PipelineID:   pipeID,
		},
		Pipeline: core.Pipeline{
			Id: pipeID,
			Blocks: []core.PipelineBlock{
				{Id: blockID, Name: "hello", Args: map[string]any{"x": 1}},
			},
		},
	}
	inv, err := InvocationFromJob(j)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Id != blockID || inv.BlockId != "hello" || inv.PipelineId != pipeID {
		t.Errorf("invocation fields mismatch: %+v", inv)
	}
	if inv.MapIndex != nil {
		t.Errorf("expected no map index")
	}
	if v := inv.Arguments["x"]; v != 1 {
		t.Errorf("args not propagated: %v", inv.Arguments)
	}
}

func TestInvocationFromJob_Mapped(t *testing.T) {
	blockID := uuid.New()
	pipeID := uuid.New()
	j := Job{
		Assignment: core.WorkerAssignment{
			InvocationID: blockID.String() + ".3",
			BlockName:    "child",
			PipelineID:   pipeID,
		},
		Pipeline: core.Pipeline{
			Id: pipeID,
			Blocks: []core.PipelineBlock{
				{Id: blockID, Name: "child"},
			},
		},
	}
	inv, err := InvocationFromJob(j)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.MapIndex == nil || *inv.MapIndex != 3 {
		t.Errorf("expected map index 3, got %v", inv.MapIndex)
	}
}

func TestInvocationFromJob_MissingBlock(t *testing.T) {
	blockID := uuid.New()
	pipeID := uuid.New()
	j := Job{
		Assignment: core.WorkerAssignment{
			InvocationID: blockID.String(),
			PipelineID:   pipeID,
		},
		Pipeline: core.Pipeline{Id: pipeID},
	}
	_, err := InvocationFromJob(j)
	if err == nil {
		t.Fatal("expected error when block is not in pipeline")
	}
}

func TestDependencyManifests_Bare(t *testing.T) {
	depID := uuid.New()
	blockID := uuid.New()
	j := Job{
		Pipeline: core.Pipeline{
			Blocks: []core.PipelineBlock{
				{Id: depID, Name: "source"},
				{Id: blockID, Name: "sink"},
			},
		},
		Manifests: map[string]core.BlockManifest{
			"source": {ID: "pkg.source"},
			"sink":   {ID: "pkg.sink"},
		},
	}
	inv := core.BlockInvocation{
		Id:      blockID,
		BlockId: "sink",
		Inputs:  []core.InputRef{{ID: depID}},
	}
	deps, err := DependencyManifests(j, inv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := deps[depID]; !ok {
		t.Fatalf("expected dep %s in map, got %+v", depID, deps)
	}
	if deps[depID].ID != "pkg.source" {
		t.Errorf("wrong manifest pulled")
	}
}

func TestDependencyManifests_Explicit(t *testing.T) {
	depID := uuid.New()
	j := Job{
		Pipeline: core.Pipeline{
			Blocks: []core.PipelineBlock{
				{Id: depID, Name: "source"},
			},
		},
		Manifests: map[string]core.BlockManifest{
			"source": {ID: "pkg.source"},
		},
	}
	inv := core.BlockInvocation{
		Inputs: []core.InputRef{{Block: &depID, Output: "out"}},
	}
	deps, err := DependencyManifests(j, inv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := deps[depID]; !ok {
		t.Fatal("dep missing")
	}
}

func TestDependencyManifests_MissingManifest(t *testing.T) {
	depID := uuid.New()
	j := Job{
		Pipeline: core.Pipeline{
			Blocks: []core.PipelineBlock{{Id: depID, Name: "source"}},
		},
		// no Manifests entry for "source"
	}
	inv := core.BlockInvocation{Inputs: []core.InputRef{{ID: depID}}}
	if _, err := DependencyManifests(j, inv); err == nil {
		t.Fatal("expected error when manifest missing")
	}
}
