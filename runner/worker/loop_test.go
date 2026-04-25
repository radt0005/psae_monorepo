package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"core"
	spade "spade_runner"
	"spade_runner/broker"

	"github.com/google/uuid"
)

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// makeInstalledWorker builds a Worker with a registry that has one
// installed fake block.  Returns the worker and a Job template pointing
// at that block.
func makeInstalledWorker(t *testing.T, fake Executor) (*Worker, spade.Job) {
	t.Helper()
	reg, root := setupRegistry(t)
	installed := filepath.Join(root, "install")
	manifest := core.BlockManifest{ID: "pkg.hello", Version: "1.0.0", Kind: core.BlockKindStandard}
	installFakeBlock(t, reg, installed, manifest, "hello")
	w := New(reg, filepath.Join(root, "work"), WithExecutor(fake))

	blockID := uuid.New()
	pipeID := uuid.New()
	job := spade.Job{
		Assignment: core.WorkerAssignment{
			InvocationID: blockID.String(),
			BlockName:    "pkg.hello",
			PipelineID:   pipeID,
			WorkDir:      filepath.Join(root, "work"),
		},
		Pipeline: core.Pipeline{
			Id:     pipeID,
			Blocks: []core.PipelineBlock{{Id: blockID, Name: "pkg.hello"}},
		},
		Manifests: map[string]core.BlockManifest{"pkg.hello": manifest},
	}
	return w, job
}

func TestRunLoop_PublishBeforeAck(t *testing.T) {
	fake := &fakeExecutor{
		result: core.BlockInvocationResult{Status: core.ExecutionStatusComplete, ExitCode: 0},
	}
	w, job := makeInstalledWorker(t, fake)

	cons := &broker.FakeConsumer{}
	cons.Enqueue(job)
	pub := &orderedPublisher{FakePublisher: &broker.FakePublisher{}, cons: cons}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunLoop(ctx, w, cons, pub, silentLogger())
	}()

	// Wait for the loop to process the job, then cancel.
	time.Sleep(150 * time.Millisecond)
	cancel()
	<-done

	if len(pub.Published) != 1 {
		t.Fatalf("expected 1 publish, got %d", len(pub.Published))
	}
	if got := cons.Settled; len(got) != 1 || got[0] != "ack" {
		t.Fatalf("expected single ack, got %v", got)
	}
	if pub.seenAckBeforePublish {
		t.Errorf("ack must happen AFTER publish, but ack-before-publish was observed")
	}
}

func TestRunLoop_InfraFailureNacksWithoutPublish(t *testing.T) {
	// Use a worker whose registry is closed so LookupBlock errors out —
	// but the test wants an INFRA failure path, so instead we set up
	// a Worker with nil registry.
	w := New(nil, "")

	cons := &broker.FakeConsumer{}
	blockID := uuid.New()
	cons.Enqueue(spade.Job{
		Assignment: core.WorkerAssignment{InvocationID: blockID.String(), BlockName: "x"},
	})
	pub := &broker.FakePublisher{}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_ = RunLoop(ctx, w, cons, pub, silentLogger())

	if len(pub.Published) != 0 {
		t.Errorf("infra failures must not publish, got %d publishes", len(pub.Published))
	}
	if got := cons.Settled; len(got) == 0 || got[0] != "nack-discard" {
		t.Errorf("expected nack-discard for infra failure, got %v", got)
	}
}

func TestRunLoop_BlockFailurePublishesErrorResult(t *testing.T) {
	fake := &fakeExecutor{
		result: core.BlockInvocationResult{
			Status: core.ExecutionStatusError, ExitCode: 2, Error: "boom",
		},
	}
	w, job := makeInstalledWorker(t, fake)

	cons := &broker.FakeConsumer{}
	cons.Enqueue(job)
	pub := &broker.FakePublisher{}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = RunLoop(ctx, w, cons, pub, silentLogger())

	if len(pub.Published) != 1 {
		t.Fatalf("block failure must still publish a result, got %d", len(pub.Published))
	}
	if got := cons.Settled; len(got) != 1 || got[0] != "ack" {
		t.Fatalf("block failure must ack the job, got %v", got)
	}
}

func TestRunLoop_MalformedPayloadNacks(t *testing.T) {
	w := &Worker{executor: coreExecutor{}}
	cons := &broker.FakeConsumer{BadJSONNext: []byte("not json")}
	pub := &broker.FakePublisher{}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = RunLoop(ctx, w, cons, pub, silentLogger())

	if len(pub.Published) != 0 {
		t.Errorf("malformed payload must not trigger publish")
	}
	if got := cons.Settled; len(got) == 0 || got[0] != "nack-discard" {
		t.Errorf("malformed payload must be nack-discarded, got %v", got)
	}
}

func TestRunLoop_PublishFailurePropagates(t *testing.T) {
	fake := &fakeExecutor{
		result: core.BlockInvocationResult{Status: core.ExecutionStatusComplete, ExitCode: 0},
	}
	w, job := makeInstalledWorker(t, fake)

	cons := &broker.FakeConsumer{}
	cons.Enqueue(job)
	pub := &broker.FakePublisher{FailWith: errors.New("broker down")}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := RunLoop(ctx, w, cons, pub, silentLogger())

	if err == nil {
		t.Fatal("expected publish failure to propagate")
	}
	if got := cons.Settled; len(got) == 0 || got[0] != "nack-discard" {
		t.Errorf("failed publish must nack the job, got %v", got)
	}
}

// orderedPublisher wraps a FakePublisher to detect whether an ack ever
// arrives BEFORE the publish completes — a protocol violation.
type orderedPublisher struct {
	*broker.FakePublisher
	cons                 *broker.FakeConsumer
	seenAckBeforePublish bool
}

func (o *orderedPublisher) Publish(ctx context.Context, result any) error {
	// Snapshot ack count before publish; if it's already non-zero,
	// an ack came through first.
	if len(o.cons.Settled) > 0 {
		o.seenAckBeforePublish = true
	}
	return o.FakePublisher.Publish(ctx, result)
}
