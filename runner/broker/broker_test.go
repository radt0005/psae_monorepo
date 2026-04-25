package broker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"core"
	spade "spade_runner"

	"github.com/google/uuid"
)

func TestMarshalRoundtripJob(t *testing.T) {
	pipeID := uuid.New()
	blockID := uuid.New()
	j := spade.Job{
		Assignment: core.WorkerAssignment{
			InvocationID: blockID.String(),
			BlockName:    "pkg.x",
			PipelineID:   pipeID,
			Args:         map[string]any{"n": float64(42)},
		},
		Pipeline: core.Pipeline{
			Id: pipeID,
			Blocks: []core.PipelineBlock{
				{Id: blockID, Name: "pkg.x"},
			},
		},
		Manifests: map[string]core.BlockManifest{"pkg.x": {ID: "pkg.x"}},
	}
	data, err := marshalJob(j)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := unmarshalJob(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Assignment.InvocationID != j.Assignment.InvocationID {
		t.Errorf("invocation id mismatch")
	}
	if got.Pipeline.Id != pipeID {
		t.Errorf("pipeline id mismatch")
	}
}

func TestMarshalResult(t *testing.T) {
	r := core.WorkerResult{InvocationID: "abc", Status: core.ExecutionStatusComplete, ExitCode: 0}
	data, err := marshalResult(r)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if !strings.Contains(string(data), `"invocation_id":"abc"`) {
		t.Errorf("result missing expected field: %s", data)
	}
}

func TestFakeConsumer_DeliversPreloadedJobs(t *testing.T) {
	f := &FakeConsumer{}
	j := spade.Job{Assignment: core.WorkerAssignment{InvocationID: "a"}}
	f.Enqueue(j)
	f.Enqueue(spade.Job{Assignment: core.WorkerAssignment{InvocationID: "b"}})

	ctx := context.Background()
	d1, err := f.Next(ctx)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if d1.Job.Assignment.InvocationID != "a" {
		t.Errorf("FIFO order violated: %s", d1.Job.Assignment.InvocationID)
	}
	if err := d1.Ack(ctx); err != nil {
		t.Errorf("Ack: %v", err)
	}
	d2, err := f.Next(ctx)
	if err != nil {
		t.Fatalf("Next 2: %v", err)
	}
	if d2.Job.Assignment.InvocationID != "b" {
		t.Errorf("second in order: got %s", d2.Job.Assignment.InvocationID)
	}
	if err := d2.Nack(ctx, false); err != nil {
		t.Errorf("Nack: %v", err)
	}
	if got := f.Settled; len(got) != 2 || got[0] != "ack" || got[1] != "nack-discard" {
		t.Errorf("wrong settlement trace: %v", got)
	}
}

func TestFakeConsumer_BlocksUntilContextCancelled(t *testing.T) {
	f := &FakeConsumer{}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := f.Next(ctx)
	if err == nil {
		t.Fatal("expected context-deadline error")
	}
}

func TestFakeConsumer_BadJSONPath(t *testing.T) {
	f := &FakeConsumer{BadJSONNext: []byte("not json")}
	d, err := f.Next(context.Background())
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if string(d.RawBody) != "not json" {
		t.Errorf("raw body not propagated: %q", d.RawBody)
	}
	// Nack path should work even without a valid Job.
	if err := d.Nack(context.Background(), false); err != nil {
		t.Errorf("Nack: %v", err)
	}
}

func TestFakePublisher_RecordsPublishes(t *testing.T) {
	p := &FakePublisher{}
	r := core.WorkerResult{InvocationID: "x"}
	if err := p.Publish(context.Background(), r); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if p.PublishedCount() != 1 {
		t.Errorf("expected 1 published, got %d", p.PublishedCount())
	}
}

func TestFakePublisher_FailInjection(t *testing.T) {
	injected := errors.New("broker down")
	p := &FakePublisher{FailWith: injected}
	err := p.Publish(context.Background(), core.WorkerResult{})
	if !errors.Is(err, injected) {
		t.Errorf("expected injected error, got %v", err)
	}
}

func TestReconnect_RespectsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Use a bogus URL; Dial will fail, but ctx is already cancelled
	// so the loop returns immediately.
	cfg := ReconnectConfig{URL: "amqp://invalid:1/", MinBackoff: time.Millisecond, MaxBackoff: time.Millisecond}
	err := Run(ctx, cfg, func(ctx context.Context, c *Conn) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestNextBackoffCaps(t *testing.T) {
	max := 5 * time.Second
	if got := nextBackoff(3*time.Second, max); got != max {
		t.Errorf("expected cap at %v, got %v", max, got)
	}
	if got := nextBackoff(1*time.Second, max); got != 2*time.Second {
		t.Errorf("expected doubling, got %v", got)
	}
}
