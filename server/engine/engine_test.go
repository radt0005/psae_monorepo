package engine

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"core"

	"github.com/google/uuid"

	"spade_server/broker"
	"spade_server/store"
)

// silentLogger returns a slog.Logger that swallows everything — useful
// to keep test output readable.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// newTestEngine returns an Engine with in-memory store + fake broker +
// in-memory manifest provider.  Returned fakes are exposed so tests can
// drive them.
func newTestEngine(t *testing.T) (*Engine, *store.MemStore, *broker.FakeJobPublisher, *broker.FakeResultConsumer, *MapManifestProvider) {
	t.Helper()
	mem := store.NewMemStore()
	pub := &broker.FakeJobPublisher{}
	cons := broker.NewFakeResultConsumer()
	mp := NewMapManifestProvider()
	eng := New(mem, pub, mp, silentLogger())
	return eng, mem, pub, cons, mp
}

func linearPipeline(t *testing.T, mp *MapManifestProvider) (core.Pipeline, []uuid.UUID) {
	t.Helper()
	mp.Set("src", core.BlockManifest{
		ID:      "test.src",
		Version: "1",
		Outputs: map[string]core.OutputDeclaration{"out": {Type: "file"}},
	})
	mp.Set("mid", core.BlockManifest{
		ID:      "test.mid",
		Version: "1",
		Inputs:  map[string]core.InputDeclaration{"in": {Type: "file"}},
		Outputs: map[string]core.OutputDeclaration{"out": {Type: "file"}},
	})
	mp.Set("snk", core.BlockManifest{
		ID:      "test.snk",
		Version: "1",
		Inputs:  map[string]core.InputDeclaration{"in": {Type: "file"}},
		Outputs: map[string]core.OutputDeclaration{"final": {Type: "file"}},
	})
	a := uuid.Must(uuid.NewV7())
	b := uuid.Must(uuid.NewV7())
	c := uuid.Must(uuid.NewV7())
	p := core.Pipeline{
		Id:      uuid.Must(uuid.NewV7()),
		Name:    "linear",
		Version: "1",
		Blocks: []core.PipelineBlock{
			{Id: a, Name: "src", Inputs: nil, Args: map[string]any{}},
			{Id: b, Name: "mid", Inputs: []core.InputRef{{ID: a}}, Args: map[string]any{}},
			{Id: c, Name: "snk", Inputs: []core.InputRef{{ID: b}}, Args: map[string]any{}},
		},
	}
	return p, []uuid.UUID{a, b, c}
}

func TestSubmitPipelinePersistsAndDispatches(t *testing.T) {
	eng, mem, pub, _, mp := newTestEngine(t)
	p, ids := linearPipeline(t, mp)

	if err := eng.SubmitPipeline(context.Background(), &p, nil, "user"); err != nil {
		t.Fatalf("SubmitPipeline: %v", err)
	}

	rec, err := mem.LoadPipeline(context.Background(), p.Id)
	if err != nil {
		t.Fatalf("LoadPipeline: %v", err)
	}
	if rec.Status != store.PipelineRunning {
		t.Errorf("status: got %s want running", rec.Status)
	}

	// dispatchSweep is event-driven; trigger it explicitly here so
	// tests don't depend on dispatchLoop running.
	if err := eng.dispatchSweep(context.Background()); err != nil {
		t.Fatalf("dispatchSweep: %v", err)
	}
	jobs := pub.PublishedJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 dispatched job, got %d", len(jobs))
	}
	if jobs[0].Assignment.BlockName != "src" {
		t.Errorf("first dispatched: %s", jobs[0].Assignment.BlockName)
	}
	if jobs[0].Assignment.PipelineID != p.Id {
		t.Errorf("PipelineID propagated incorrectly")
	}
	// Persisted invocation rows: 3 pending + 1 dispatched override on src.
	invs, _ := mem.LoadInvocations(context.Background(), p.Id)
	if len(invs) != 3 {
		t.Fatalf("expected 3 invocation rows, got %d", len(invs))
	}
	var dispatched int
	for _, inv := range invs {
		if inv.Status == store.InvocationDispatched {
			dispatched++
			if inv.BlockID != ids[0] {
				t.Errorf("dispatched row wrong: %s", inv.BlockID)
			}
		}
	}
	if dispatched != 1 {
		t.Errorf("expected exactly 1 dispatched row, got %d", dispatched)
	}
}

func TestApplyResultAdvancesPipeline(t *testing.T) {
	eng, mem, pub, _, mp := newTestEngine(t)
	p, ids := linearPipeline(t, mp)

	ctx := context.Background()
	if err := eng.SubmitPipeline(ctx, &p, nil, ""); err != nil {
		t.Fatal(err)
	}
	_ = eng.dispatchSweep(ctx)

	// Complete src.
	wr := core.WorkerResult{
		InvocationID: ids[0].String(),
		PipelineID:   p.Id,
		Status:       core.ExecutionStatusComplete,
		ExitCode:     0,
	}
	if err := eng.applyResult(ctx, wr); err != nil {
		t.Fatalf("applyResult: %v", err)
	}
	_ = eng.dispatchSweep(ctx)
	jobs := pub.PublishedJobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 dispatched after src complete, got %d", len(jobs))
	}
	if jobs[1].Assignment.BlockName != "mid" {
		t.Errorf("second dispatch: %s", jobs[1].Assignment.BlockName)
	}

	// Complete mid; then snk should fire.
	wr.InvocationID = ids[1].String()
	if err := eng.applyResult(ctx, wr); err != nil {
		t.Fatal(err)
	}
	_ = eng.dispatchSweep(ctx)
	jobs = pub.PublishedJobs()
	if len(jobs) != 3 {
		t.Fatalf("expected 3 dispatched, got %d", len(jobs))
	}
	if jobs[2].Assignment.BlockName != "snk" {
		t.Errorf("third dispatch: %s", jobs[2].Assignment.BlockName)
	}

	// Complete snk; pipeline should be marked complete.
	wr.InvocationID = ids[2].String()
	if err := eng.applyResult(ctx, wr); err != nil {
		t.Fatal(err)
	}
	rec, _ := mem.LoadPipeline(ctx, p.Id)
	if rec.Status != store.PipelineComplete {
		t.Errorf("pipeline status: %s", rec.Status)
	}
}

func TestApplyResultFailureHaltsPipeline(t *testing.T) {
	eng, mem, _, _, mp := newTestEngine(t)
	p, ids := linearPipeline(t, mp)

	ctx := context.Background()
	if err := eng.SubmitPipeline(ctx, &p, nil, ""); err != nil {
		t.Fatal(err)
	}
	_ = eng.dispatchSweep(ctx)

	// src completes successfully.
	if err := eng.applyResult(ctx, core.WorkerResult{
		InvocationID: ids[0].String(), PipelineID: p.Id, Status: core.ExecutionStatusComplete,
	}); err != nil {
		t.Fatal(err)
	}
	_ = eng.dispatchSweep(ctx)
	// mid fails.
	if err := eng.applyResult(ctx, core.WorkerResult{
		InvocationID: ids[1].String(), PipelineID: p.Id, Status: core.ExecutionStatusError,
		ExitCode: 1, Error: "boom",
	}); err != nil {
		t.Fatal(err)
	}
	rec, _ := mem.LoadPipeline(ctx, p.Id)
	if rec.Status != store.PipelineFailed {
		t.Errorf("status: got %s want failed", rec.Status)
	}
	// snk must NOT have been dispatched.
	pub := newTestPub(eng)
	if pub != nil {
		// (no-op; left for symmetry)
	}
}

// newTestPub is a tiny helper that pulls the publisher out of the engine
// via type assertion.  Returns nil if it isn't a FakeJobPublisher.
func newTestPub(e *Engine) *broker.FakeJobPublisher {
	if p, ok := e.publisher.(*broker.FakeJobPublisher); ok {
		return p
	}
	return nil
}

func TestApplyResultDuplicateIgnored(t *testing.T) {
	eng, _, _, _, mp := newTestEngine(t)
	p, ids := linearPipeline(t, mp)
	ctx := context.Background()
	if err := eng.SubmitPipeline(ctx, &p, nil, ""); err != nil {
		t.Fatal(err)
	}
	wr := core.WorkerResult{InvocationID: ids[0].String(), PipelineID: p.Id, Status: core.ExecutionStatusComplete}
	if err := eng.applyResult(ctx, wr); err != nil {
		t.Fatal(err)
	}
	// Repeat with mutated exit code; should be ignored.
	wr.ExitCode = 99
	if err := eng.applyResult(ctx, wr); err != nil {
		t.Fatal(err)
	}
}

func TestCancelPipelineMarksCancelled(t *testing.T) {
	eng, mem, _, _, mp := newTestEngine(t)
	p, _ := linearPipeline(t, mp)
	ctx := context.Background()
	if err := eng.SubmitPipeline(ctx, &p, nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := eng.CancelPipeline(ctx, p.Id); err != nil {
		t.Fatal(err)
	}
	rec, _ := mem.LoadPipeline(ctx, p.Id)
	if rec.Status != store.PipelineCancelled {
		t.Errorf("status: %s", rec.Status)
	}
}

func TestValidationErrorOnUnknownBlock(t *testing.T) {
	eng, _, _, _, _ := newTestEngine(t)
	// Empty manifest provider — every block reference is unknown.
	p := core.Pipeline{
		Id:      uuid.Must(uuid.NewV7()),
		Name:    "p",
		Version: "1",
		Blocks: []core.PipelineBlock{
			{Id: uuid.Must(uuid.NewV7()), Name: "bogus", Inputs: nil, Args: map[string]any{}},
		},
	}
	err := eng.SubmitPipeline(context.Background(), &p, nil, "")
	if err == nil {
		t.Fatal("expected validation error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) || len(ve.Errors) == 0 {
		t.Errorf("expected *ValidationError, got %v", err)
	}
}

func TestPipelineStatusSnapshot(t *testing.T) {
	eng, _, _, _, mp := newTestEngine(t)
	p, ids := linearPipeline(t, mp)
	ctx := context.Background()
	if err := eng.SubmitPipeline(ctx, &p, nil, ""); err != nil {
		t.Fatal(err)
	}
	view, err := eng.PipelineStatus(ctx, p.Id)
	if err != nil {
		t.Fatal(err)
	}
	if view.ID != p.Id {
		t.Errorf("ID mismatch")
	}
	if len(view.Blocks) != 3 {
		t.Fatalf("expected 3 blocks in snapshot, got %d", len(view.Blocks))
	}
	for _, bs := range view.Blocks {
		if bs.BlockID == ids[0] && bs.Status != core.BlockSnapshotExecutable {
			t.Errorf("source block: got %s want executable", bs.Status)
		}
	}
}

func TestRecoverFromStore(t *testing.T) {
	mem := store.NewMemStore()
	mp := NewMapManifestProvider()
	p, ids := linearPipeline(t, mp)
	// Pre-populate store as if a previous engine had run src to
	// completion and been killed.
	yamlBytes := mustMarshalPipeline(t, p)
	rec := store.PipelineRecord{
		ID:          p.Id,
		Name:        p.Name,
		Version:     p.Version,
		YAML:        string(yamlBytes),
		Status:      store.PipelineRunning,
		SubmittedAt: time.Now().UTC().Add(-time.Minute),
	}
	ctx := context.Background()
	if err := mem.InsertPipeline(ctx, rec); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, pb := range p.Blocks {
		st := store.InvocationPending
		var done *time.Time
		if pb.Id == ids[0] {
			st = store.InvocationComplete
			done = &now
		}
		_ = mem.UpsertInvocation(ctx, store.InvocationRecord{
			ID: pb.Id.String(), PipelineID: p.Id, BlockID: pb.Id,
			BlockName: pb.Name, Status: st, CompletedAt: done,
		})
	}
	eng := New(mem, &broker.FakeJobPublisher{}, mp, silentLogger())
	if err := eng.Recover(ctx); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	// After recovery, dispatching should send mid (src already done).
	if err := eng.dispatchSweep(ctx); err != nil {
		t.Fatal(err)
	}
	pub := newTestPub(eng)
	if pub == nil {
		t.Fatal("publisher type assertion failed")
	}
	jobs := pub.PublishedJobs()
	if len(jobs) != 1 {
		t.Fatalf("after recover: expected 1 dispatch, got %d", len(jobs))
	}
	if jobs[0].Assignment.BlockName != "mid" {
		t.Errorf("expected mid dispatched after recovery, got %s", jobs[0].Assignment.BlockName)
	}
}

func mustMarshalPipeline(t *testing.T, p core.Pipeline) []byte {
	t.Helper()
	out, err := yamlMarshalPipeline(p)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// singleBlockPipeline builds a one-block pipeline for callback tests.
func singleBlockPipeline(t *testing.T, mp *MapManifestProvider) (core.Pipeline, uuid.UUID) {
	t.Helper()
	mp.Set("solo", core.BlockManifest{
		ID:      "test.solo",
		Version: "1",
		Outputs: map[string]core.OutputDeclaration{"out": {Type: "file"}},
	})
	blockID := uuid.Must(uuid.NewV7())
	p := core.Pipeline{
		Id:      uuid.Must(uuid.NewV7()),
		Name:    "solo",
		Version: "1",
		Blocks: []core.PipelineBlock{
			{Id: blockID, Name: "solo", Inputs: nil, Args: map[string]any{}},
		},
	}
	return p, blockID
}

func TestEngine_CallbackFiredOnComplete(t *testing.T) {
	eng, _, _, _, mp := newTestEngine(t)
	p, blockID := singleBlockPipeline(t, mp)
	ctx := context.Background()

	var mu sync.Mutex
	var gotPayloads []CallbackPayload
	eng.SetCallback(func(_ context.Context, _ uuid.UUID, payload CallbackPayload) {
		mu.Lock()
		gotPayloads = append(gotPayloads, payload)
		mu.Unlock()
	})

	if err := eng.SubmitPipeline(ctx, &p, nil, "user"); err != nil {
		t.Fatalf("SubmitPipeline: %v", err)
	}
	if err := eng.dispatchSweep(ctx); err != nil {
		t.Fatalf("dispatchSweep: %v", err)
	}
	res := core.WorkerResult{
		InvocationID: blockID.String(),
		PipelineID:   p.Id,
		Status:       core.ExecutionStatusComplete,
	}
	if err := eng.applyResult(ctx, res); err != nil {
		t.Fatalf("applyResult: %v", err)
	}

	mu.Lock()
	n := len(gotPayloads)
	mu.Unlock()
	if n != 1 {
		t.Fatalf("expected 1 callback, got %d", n)
	}
	if gotPayloads[0].Status != store.PipelineComplete {
		t.Errorf("callback status: got %q, want %q", gotPayloads[0].Status, store.PipelineComplete)
	}
}

func TestEngine_CallbackFiredOnFailure(t *testing.T) {
	eng, _, _, _, mp := newTestEngine(t)
	p, blockID := singleBlockPipeline(t, mp)
	ctx := context.Background()

	var gotPayload CallbackPayload
	var called bool
	eng.SetCallback(func(_ context.Context, _ uuid.UUID, payload CallbackPayload) {
		called = true
		gotPayload = payload
	})

	if err := eng.SubmitPipeline(ctx, &p, nil, "user"); err != nil {
		t.Fatalf("SubmitPipeline: %v", err)
	}
	if err := eng.dispatchSweep(ctx); err != nil {
		t.Fatalf("dispatchSweep: %v", err)
	}
	res := core.WorkerResult{
		InvocationID: blockID.String(),
		PipelineID:   p.Id,
		Status:       core.ExecutionStatusError,
		Error:        "block crashed",
	}
	if err := eng.applyResult(ctx, res); err != nil {
		t.Fatalf("applyResult: %v", err)
	}

	if !called {
		t.Fatal("callback not called after pipeline failure")
	}
	if gotPayload.Status != store.PipelineFailed {
		t.Errorf("callback status: got %q, want %q", gotPayload.Status, store.PipelineFailed)
	}
	if gotPayload.ErrorMsg != "block crashed" {
		t.Errorf("callback error: got %q, want 'block crashed'", gotPayload.ErrorMsg)
	}
}

func TestEngine_CallbackFiredOnCancel(t *testing.T) {
	eng, _, _, _, mp := newTestEngine(t)
	p, _ := singleBlockPipeline(t, mp)
	ctx := context.Background()

	var gotPayload CallbackPayload
	var called bool
	eng.SetCallback(func(_ context.Context, _ uuid.UUID, payload CallbackPayload) {
		called = true
		gotPayload = payload
	})

	if err := eng.SubmitPipeline(ctx, &p, nil, "user"); err != nil {
		t.Fatalf("SubmitPipeline: %v", err)
	}
	if err := eng.CancelPipeline(ctx, p.Id); err != nil {
		t.Fatalf("CancelPipeline: %v", err)
	}

	if !called {
		t.Fatal("callback not called after cancel")
	}
	if gotPayload.Status != store.PipelineCancelled {
		t.Errorf("callback status: got %q, want %q", gotPayload.Status, store.PipelineCancelled)
	}
}

func TestEngine_CallbackNotFiredMidPipeline(t *testing.T) {
	eng, _, _, _, mp := newTestEngine(t)
	p, ids := linearPipeline(t, mp)
	ctx := context.Background()

	var callCount int
	eng.SetCallback(func(_ context.Context, _ uuid.UUID, _ CallbackPayload) {
		callCount++
	})

	if err := eng.SubmitPipeline(ctx, &p, nil, "user"); err != nil {
		t.Fatalf("SubmitPipeline: %v", err)
	}
	// Complete only the first block of a 3-block pipeline — no terminal state yet.
	if err := eng.dispatchSweep(ctx); err != nil {
		t.Fatalf("dispatchSweep: %v", err)
	}
	res := core.WorkerResult{
		InvocationID: ids[0].String(),
		PipelineID:   p.Id,
		Status:       core.ExecutionStatusComplete,
	}
	if err := eng.applyResult(ctx, res); err != nil {
		t.Fatalf("applyResult: %v", err)
	}

	if callCount != 0 {
		t.Errorf("callback fired after intermediate block completion; want 0 calls, got %d", callCount)
	}
}
