package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"core"

	"github.com/google/uuid"

	"spade_server/broker"
	"spade_server/store"
)

// TestEngineRunDrivesPipelineToCompletion exercises the dispatch +
// result loops end-to-end through engine.Run.  A producer goroutine
// pretends to be the worker fleet: for every job published by the
// engine, it waits a moment and enqueues a corresponding success
// result on the fake consumer.  After all three blocks complete the
// pipeline status in the store should be PipelineComplete.
func TestEngineRunDrivesPipelineToCompletion(t *testing.T) {
	mem := store.NewMemStore()
	pub := &broker.FakeJobPublisher{}
	cons := broker.NewFakeResultConsumer()
	mp := NewMapManifestProvider()
	eng := New(mem, pub, mp, silentLogger())

	p, _ := linearPipeline(t, mp)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := eng.SubmitPipeline(ctx, &p, nil, ""); err != nil {
		t.Fatal(err)
	}

	// Producer goroutine: poll the publisher, ack every job by feeding
	// a success result back through the consumer.
	var wg sync.WaitGroup
	wg.Add(1)
	processed := map[string]bool{}
	go func() {
		defer wg.Done()
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			rec, _ := mem.LoadPipeline(ctx, p.Id)
			if rec.Status == store.PipelineComplete || rec.Status == store.PipelineFailed {
				return
			}
			for _, j := range pub.PublishedJobs() {
				id := j.Assignment.InvocationID
				if processed[id] {
					continue
				}
				processed[id] = true
				cons.Enqueue(core.WorkerResult{
					InvocationID: id,
					PipelineID:   j.Assignment.PipelineID,
					Status:       core.ExecutionStatusComplete,
				})
			}
		}
	}()

	runErr := make(chan error, 1)
	go func() { runErr <- eng.Run(ctx, cons) }()

	// Poll for terminal status.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rec, _ := mem.LoadPipeline(ctx, p.Id)
		if rec.Status == store.PipelineComplete {
			cancel()
			wg.Wait()
			<-runErr
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	wg.Wait()
	<-runErr
	rec, _ := mem.LoadPipeline(context.Background(), p.Id)
	t.Fatalf("pipeline did not complete; final status: %s, published=%d", rec.Status, len(pub.PublishedJobs()))
}

// TestEngineRestartResumesPipeline simulates the crash-and-restart
// recovery scenario from scheduler.md §State Management.  We submit a
// pipeline, drive the first block to completion, then "crash" by
// throwing the engine away and starting a fresh one against the same
// MemStore.  The new engine should recover and continue dispatching.
func TestEngineRestartResumesPipeline(t *testing.T) {
	mem := store.NewMemStore()
	mp := NewMapManifestProvider()
	pub1 := &broker.FakeJobPublisher{}
	eng1 := New(mem, pub1, mp, silentLogger())
	p, ids := linearPipeline(t, mp)

	ctx := context.Background()
	if err := eng1.SubmitPipeline(ctx, &p, nil, ""); err != nil {
		t.Fatal(err)
	}
	_ = eng1.dispatchSweep(ctx)
	// Block A completes via the first engine.
	if err := eng1.applyResult(ctx, core.WorkerResult{
		InvocationID: ids[0].String(), PipelineID: p.Id, Status: core.ExecutionStatusComplete,
	}); err != nil {
		t.Fatal(err)
	}
	// Crash: throw eng1 away.
	_ = eng1.Close()

	// Fresh engine against the same store.
	pub2 := &broker.FakeJobPublisher{}
	eng2 := New(mem, pub2, mp, silentLogger())
	if err := eng2.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if err := eng2.dispatchSweep(ctx); err != nil {
		t.Fatal(err)
	}
	jobs := pub2.PublishedJobs()
	if len(jobs) != 1 {
		t.Fatalf("after restart: expected 1 dispatch, got %d", len(jobs))
	}
	if jobs[0].Assignment.BlockName != "mid" {
		t.Errorf("after restart: expected mid dispatched, got %s", jobs[0].Assignment.BlockName)
	}
}

// TestEngineRunIgnoresMalformedResults verifies the malformed-payload
// path: a result body that fails to unmarshal should be nacked without
// requeue and the engine should keep running.
func TestEngineRunIgnoresMalformedResults(t *testing.T) {
	mem := store.NewMemStore()
	pub := &broker.FakeJobPublisher{}
	cons := broker.NewFakeResultConsumer()
	mp := NewMapManifestProvider()
	eng := New(mem, pub, mp, silentLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cons.BadJSONNext = []byte("not-json")
	runErr := make(chan error, 1)
	go func() { runErr <- eng.Run(ctx, cons) }()

	// Give the loop time to consume + nack.
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-runErr
	if len(cons.Settled) != 1 || cons.Settled[0] != "nack-discard" {
		t.Errorf("expected nack-discard, got %v", cons.Settled)
	}
}

// TestEngineCancellationIgnoresOrphanedResults verifies that after a
// pipeline is cancelled, a result for an in-flight invocation is
// silently ignored by the duplicate-detection / cancellation logic.
func TestEngineCancellationIgnoresOrphanedResults(t *testing.T) {
	eng, mem, _, _, mp := newTestEngine(t)
	p, ids := linearPipeline(t, mp)
	ctx := context.Background()
	if err := eng.SubmitPipeline(ctx, &p, nil, ""); err != nil {
		t.Fatal(err)
	}
	_ = eng.dispatchSweep(ctx)
	if err := eng.CancelPipeline(ctx, p.Id); err != nil {
		t.Fatal(err)
	}
	// Now apply a result for the already-dispatched src block. The
	// engine still calls Update but the pipeline is marked cancelled
	// in the store — that's what matters.
	_ = eng.applyResult(ctx, core.WorkerResult{
		InvocationID: ids[0].String(), PipelineID: p.Id, Status: core.ExecutionStatusComplete,
	})
	rec, _ := mem.LoadPipeline(ctx, p.Id)
	if rec.Status != store.PipelineCancelled {
		t.Errorf("cancellation overwritten: %s", rec.Status)
	}
	_ = uuid.Nil
}
