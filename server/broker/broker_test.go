package broker

import (
	"context"
	"testing"

	"core"

	spade "spade_runner"

	"github.com/google/uuid"
)

func TestFakeJobPublisherRoundTrip(t *testing.T) {
	pub := &FakeJobPublisher{}
	defer pub.Close(context.Background())

	job := spade.Job{
		Assignment: core.WorkerAssignment{
			InvocationID: uuid.Must(uuid.NewV7()).String(),
			BlockName:    "demo",
			PipelineID:   uuid.Must(uuid.NewV7()),
		},
	}
	if err := pub.Publish(context.Background(), job); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	got := pub.PublishedJobs()
	if len(got) != 1 {
		t.Fatalf("expected 1 published job, got %d", len(got))
	}
	if got[0].Assignment.BlockName != "demo" {
		t.Errorf("payload mismatch")
	}
}

func TestFakeResultConsumerOrdering(t *testing.T) {
	c := NewFakeResultConsumer()
	defer c.Close(context.Background())
	pid := uuid.Must(uuid.NewV7())
	c.Enqueue(core.WorkerResult{InvocationID: "a", PipelineID: pid, Status: core.ExecutionStatusComplete})
	c.Enqueue(core.WorkerResult{InvocationID: "b", PipelineID: pid, Status: core.ExecutionStatusComplete})

	ctx := context.Background()
	d1, err := c.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d1.Result.InvocationID != "a" {
		t.Fatalf("first delivery wrong: %s", d1.Result.InvocationID)
	}
	if err := d1.Ack(ctx); err != nil {
		t.Fatalf("ack: %v", err)
	}

	d2, err := c.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d2.Result.InvocationID != "b" {
		t.Fatalf("second delivery wrong: %s", d2.Result.InvocationID)
	}
	if err := d2.Nack(ctx, false); err != nil {
		t.Fatalf("nack: %v", err)
	}

	if len(c.Settled) != 2 || c.Settled[0] != "ack" || c.Settled[1] != "nack-discard" {
		t.Fatalf("settled mismatch: %v", c.Settled)
	}
}

func TestFakeResultConsumerBadJSON(t *testing.T) {
	c := NewFakeResultConsumer()
	c.BadJSONNext = []byte("not-json")
	d, err := c.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if d.Result.InvocationID != "" {
		t.Errorf("expected zero-valued result, got %+v", d.Result)
	}
	if string(d.RawBody) != "not-json" {
		t.Errorf("raw body not propagated")
	}
}
