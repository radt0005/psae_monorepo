package spade_runner

import (
	"testing"

	"core"

	"github.com/google/uuid"
)

func TestBuildJobRoundTrip(t *testing.T) {
	a := uuid.Must(uuid.NewV7())
	b := uuid.Must(uuid.NewV7())
	pipeline := core.Pipeline{
		Id:      uuid.Must(uuid.NewV7()),
		Name:    "tp",
		Version: "1.0",
		Blocks: []core.PipelineBlock{
			{Id: a, Name: "src", Inputs: nil, Args: map[string]any{}},
			{Id: b, Name: "mid", Inputs: []core.InputRef{{ID: a}}, Args: map[string]any{}},
		},
	}
	manifests := map[string]core.BlockManifest{
		"src": {ID: "coll.src", Version: "1.0", Outputs: map[string]core.OutputDeclaration{"data": {Type: "file"}}},
		"mid": {ID: "coll.mid", Version: "1.0", Inputs: map[string]core.InputDeclaration{"data": {Type: "file"}}},
	}
	inv := core.BlockInvocation{
		Id:         b,
		BlockId:    "mid",
		PipelineId: pipeline.Id,
		Inputs:     []core.InputRef{{ID: a}},
		Arguments:  map[string]any{},
	}
	job := BuildJobForInvocation(inv, pipeline, manifests, "/tmp/work")
	if job.Assignment.InvocationID != inv.InvocationID() {
		t.Fatalf("InvocationID mismatch: got %s want %s", job.Assignment.InvocationID, inv.InvocationID())
	}
	if job.Assignment.WorkDir != "/tmp/work" {
		t.Fatalf("WorkDir not propagated")
	}
	if len(job.Manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(job.Manifests))
	}
	// Round-trip back through InvocationFromJob.
	got, err := InvocationFromJob(job)
	if err != nil {
		t.Fatalf("InvocationFromJob: %v", err)
	}
	if got.Id != b || got.BlockId != "mid" {
		t.Fatalf("round-trip changed invocation: %+v", got)
	}
}

func TestBuildJobCarriesPinnedCollectionVersion(t *testing.T) {
	a := uuid.Must(uuid.NewV7())
	pipeline := core.Pipeline{
		Id:   uuid.Must(uuid.NewV7()),
		Name: "tp",
		Blocks: []core.PipelineBlock{
			{Id: a, Name: "src", Version: "2.3.1", Args: map[string]any{}},
		},
	}
	inv := core.BlockInvocation{Id: a, BlockId: "coll.src", PipelineId: pipeline.Id, Arguments: map[string]any{}}

	// The owning block's pinned version reaches the assignment (Option A).
	job := BuildJobForInvocation(inv, pipeline, nil, "")
	if job.Assignment.CollectionVersion != "2.3.1" {
		t.Fatalf("CollectionVersion = %q, want 2.3.1", job.Assignment.CollectionVersion)
	}

	// Back-compat: an unpinned block yields an empty version (legacy lookup).
	pipeline.Blocks[0].Version = ""
	job = BuildJobForInvocation(inv, pipeline, nil, "")
	if job.Assignment.CollectionVersion != "" {
		t.Fatalf("unpinned block must leave CollectionVersion empty, got %q", job.Assignment.CollectionVersion)
	}
}
