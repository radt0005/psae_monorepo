package core

import (
	"testing"

	"github.com/google/uuid"
)

func makeServerLinearPipeline(t *testing.T) (Pipeline, []uuid.UUID) {
	t.Helper()
	a := uuid.Must(uuid.NewV7())
	b := uuid.Must(uuid.NewV7())
	c := uuid.Must(uuid.NewV7())
	p := Pipeline{
		Id:      uuid.Must(uuid.NewV7()),
		Name:    "linear",
		Version: "1.0",
		Blocks: []PipelineBlock{
			{Id: a, Name: "src", Inputs: nil, Args: map[string]any{}},
			{Id: b, Name: "mid", Inputs: []InputRef{{ID: a}}, Args: map[string]any{}},
			{Id: c, Name: "snk", Inputs: []InputRef{{ID: b}}, Args: map[string]any{}},
		},
	}
	return p, []uuid.UUID{a, b, c}
}

func TestDrainReturnsAndClearsExecutable(t *testing.T) {
	p, ids := makeServerLinearPipeline(t)

	mts := &MultiTenantScheduler{
		Pipelines:         map[uuid.UUID]Pipeline{},
		Schedulers:        map[uuid.UUID]*SinglePipelineScheduler{},
		Workers:           map[uuid.UUID]Worker{},
		CurrentExecutions: map[uuid.UUID]BlockInvocation{},
	}
	if err := mts.AddPipeline(p); err != nil {
		t.Fatalf("AddPipeline: %v", err)
	}

	drained := mts.Drain()
	if len(drained) != 1 {
		t.Fatalf("expected 1 source block drained, got %d", len(drained))
	}
	if drained[0].Id != ids[0] {
		t.Fatalf("drained wrong block: got %s want %s", drained[0].Id, ids[0])
	}
	// Subsequent Drain returns nothing until a result lands.
	if again := mts.Drain(); len(again) != 0 {
		t.Fatalf("expected empty drain after first, got %d", len(again))
	}
}

func TestIsAlreadyProcessed(t *testing.T) {
	p, ids := makeServerLinearPipeline(t)

	mts := &MultiTenantScheduler{
		Pipelines:         map[uuid.UUID]Pipeline{},
		Schedulers:        map[uuid.UUID]*SinglePipelineScheduler{},
		Workers:           map[uuid.UUID]Worker{},
		CurrentExecutions: map[uuid.UUID]BlockInvocation{},
	}
	if err := mts.AddPipeline(p); err != nil {
		t.Fatal(err)
	}
	if mts.IsAlreadyProcessed(ids[0].String()) {
		t.Fatal("nothing has finished yet")
	}
	// Complete the source block.
	res := BlockInvocationResult{
		Id:         ids[0],
		PipelineId: p.Id,
		Status:     ExecutionStatusComplete,
	}
	if err := mts.Update(ids[0], res); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !mts.IsAlreadyProcessed(ids[0].String()) {
		t.Fatal("expected IsAlreadyProcessed=true after Update")
	}
}

func TestRehydrateReplaysResults(t *testing.T) {
	p, ids := makeServerLinearPipeline(t)
	// First run: complete block A.
	results := []BlockInvocationResult{
		{Id: ids[0], PipelineId: p.Id, Status: ExecutionStatusComplete},
	}

	mts := &MultiTenantScheduler{
		Pipelines:         map[uuid.UUID]Pipeline{},
		Schedulers:        map[uuid.UUID]*SinglePipelineScheduler{},
		Workers:           map[uuid.UUID]Worker{},
		CurrentExecutions: map[uuid.UUID]BlockInvocation{},
	}
	if err := mts.Rehydrate(p, results); err != nil {
		t.Fatalf("Rehydrate: %v", err)
	}
	// After rehydration block B should be executable (its dep is complete).
	drained := mts.Drain()
	if len(drained) != 1 || drained[0].Id != ids[1] {
		t.Fatalf("expected B executable after rehydrate, got %+v", drained)
	}
}

func TestSnapshotReportsBlockStatus(t *testing.T) {
	p, ids := makeServerLinearPipeline(t)
	mts := &MultiTenantScheduler{
		Pipelines:         map[uuid.UUID]Pipeline{},
		Schedulers:        map[uuid.UUID]*SinglePipelineScheduler{},
		Workers:           map[uuid.UUID]Worker{},
		CurrentExecutions: map[uuid.UUID]BlockInvocation{},
	}
	if err := mts.AddPipeline(p); err != nil {
		t.Fatal(err)
	}
	snap, ok := mts.Snapshot(p.Id)
	if !ok {
		t.Fatal("expected snapshot")
	}
	if len(snap.Blocks) != 3 {
		t.Fatalf("expected 3 blocks in snapshot, got %d", len(snap.Blocks))
	}
	// Block A (source) should be executable; others pending.
	for _, bs := range snap.Blocks {
		switch bs.BlockID {
		case ids[0]:
			if bs.Status != BlockSnapshotExecutable {
				t.Errorf("block A: expected executable, got %s", bs.Status)
			}
		case ids[1], ids[2]:
			if bs.Status != BlockSnapshotPending {
				t.Errorf("block %s: expected pending, got %s", bs.BlockID, bs.Status)
			}
		}
	}
}

func TestWorkerResultToInvocationResult(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	pid := uuid.Must(uuid.NewV7())
	wr := WorkerResult{
		InvocationID: id.String(),
		PipelineID:   pid,
		Status:       ExecutionStatusComplete,
		ExitCode:     0,
		LogsPath:     "/work/logs",
	}
	r := WorkerResultToInvocationResult(wr)
	if r.Id != id {
		t.Fatalf("Id: got %s want %s", r.Id, id)
	}
	if r.PipelineId != pid {
		t.Fatalf("PipelineId mismatch")
	}
	if r.Status != ExecutionStatusComplete {
		t.Fatalf("Status mismatch")
	}
	if r.LogsPath != "/work/logs" {
		t.Fatalf("LogsPath mismatch")
	}
}

func TestParseInvocationIDForScheduler(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	// Plain UUID.
	u, mi, err := parseInvocationIDForScheduler(id.String())
	if err != nil || u != id || mi != nil {
		t.Fatalf("plain: got %v %v %v", u, mi, err)
	}
	// With map index.
	u, mi, err = parseInvocationIDForScheduler(id.String() + ".7")
	if err != nil || u != id || mi == nil || *mi != 7 {
		t.Fatalf("indexed: got %v %v %v", u, mi, err)
	}
}
