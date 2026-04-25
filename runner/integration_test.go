//go:build integration

// Package spade_runner integration tests.  These exercise the full
// worker pipeline end-to-end against the real core.Execute (which runs
// block subprocesses under the isolate sandbox).  They are only built
// when `-tags integration` is passed and require:
//
//   - Ubuntu `isolate` installed on $PATH.
//   - `go` on $PATH (so the hello-go fixture can be compiled).
//
// The fake AMQP consumer/publisher stand in for a real RabbitMQ broker —
// that lives in the separate broker_integration_test if/when added.
package spade_runner_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"core"
	spade "spade_runner"
	"spade_runner/broker"
	"spade_runner/testutil"
	"spade_runner/worker"

	"github.com/google/uuid"
)

func requireIsolate(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("isolate"); err != nil {
		t.Skip("isolate binary not installed, skipping integration test")
	}
}

// TestEndToEnd_HelloBlock drives a real "hello.hello" block execution
// through the full worker.RunLoop → core.Execute → isolate → result
// publish → ack pipeline.
func TestEndToEnd_HelloBlock(t *testing.T) {
	requireIsolate(t)

	dir := testutil.IsolateFriendlyTempDir(t)
	dbPath := filepath.Join(dir, "registry.db")
	reg, err := core.OpenRegistry(dbPath)
	if err != nil {
		t.Fatalf("OpenRegistry: %v", err)
	}
	defer reg.Close()

	testutil.InstallHelloFixture(t, reg)

	workRoot := filepath.Join(dir, "work")
	w := worker.New(reg, workRoot)

	blockID := uuid.New()
	pipeID := uuid.New()
	manifest, err := core.LoadBlockManifest(filepath.Join(testutil.FixtureRoot(), "hello-go", "blocks", "hello.yaml"))
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	job := spade.Job{
		Assignment: core.WorkerAssignment{
			InvocationID: blockID.String(),
			BlockName:    manifest.ID,
			PipelineID:   pipeID,
			WorkDir:      workRoot,
		},
		Pipeline: core.Pipeline{
			Id:     pipeID,
			Blocks: []core.PipelineBlock{{Id: blockID, Name: manifest.ID}},
		},
		Manifests: map[string]core.BlockManifest{manifest.ID: manifest},
	}

	cons := &broker.FakeConsumer{}
	cons.Enqueue(job)
	pub := &broker.FakePublisher{}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// RunLoop will process the single queued job, publish a result,
	// ack, and then block on Next.  We cancel to unblock after the
	// publish.
	done := make(chan struct{})
	go func() {
		_ = worker.RunLoop(ctx, w, cons, pub, testutil.SilentLogger())
		close(done)
	}()

	// Wait for the publish to appear, then cancel.
	deadline := time.After(25 * time.Second)
	for pub.PublishedCount() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for result publish")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
	cancel()
	<-done

	if pub.PublishedCount() != 1 {
		t.Fatalf("expected 1 publish, got %d", pub.PublishedCount())
	}
	body := string(pub.Published[0])
	if !strings.Contains(body, `"status":"complete"`) {
		t.Errorf("expected successful status, got %s", body)
	}
	if len(cons.Settled) != 1 || cons.Settled[0] != "ack" {
		t.Errorf("expected job to be acked after successful publish, got %v", cons.Settled)
	}
}

// TestEndToEnd_BrokenBlock verifies that a block exiting non-zero is
// reported as a block failure — the result is still published, the job
// is still acked, and Error carries the exit code.
func TestEndToEnd_BrokenBlock(t *testing.T) {
	requireIsolate(t)

	dir := testutil.IsolateFriendlyTempDir(t)
	dbPath := filepath.Join(dir, "registry.db")
	reg, err := core.OpenRegistry(dbPath)
	if err != nil {
		t.Fatalf("OpenRegistry: %v", err)
	}
	defer reg.Close()

	testutil.InstallHelloFixture(t, reg)

	workRoot := filepath.Join(dir, "work")
	w := worker.New(reg, workRoot)

	blockID := uuid.New()
	pipeID := uuid.New()
	manifest, err := core.LoadBlockManifest(filepath.Join(testutil.FixtureRoot(), "hello-go", "blocks", "broken.yaml"))
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	job := spade.Job{
		Assignment: core.WorkerAssignment{
			InvocationID: blockID.String(),
			BlockName:    manifest.ID,
			PipelineID:   pipeID,
			WorkDir:      workRoot,
		},
		Pipeline: core.Pipeline{
			Id:     pipeID,
			Blocks: []core.PipelineBlock{{Id: blockID, Name: manifest.ID}},
		},
		Manifests: map[string]core.BlockManifest{manifest.ID: manifest},
	}

	cons := &broker.FakeConsumer{}
	cons.Enqueue(job)
	pub := &broker.FakePublisher{}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = worker.RunLoop(ctx, w, cons, pub, testutil.SilentLogger())
		close(done)
	}()

	deadline := time.After(25 * time.Second)
	for pub.PublishedCount() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for result publish")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
	cancel()
	<-done

	if pub.PublishedCount() != 1 {
		t.Fatalf("expected 1 publish, got %d", pub.PublishedCount())
	}
	body := string(pub.Published[0])
	if !strings.Contains(body, `"status":"error"`) {
		t.Errorf("expected error status, got %s", body)
	}
	// Exit code: isolate reports its wrapper exit code (1 for any non-zero
	// inner), and surfaces the inner exit in stderr as "Exited with error
	// status <n>".  So we assert both the non-zero exit and the stderr
	// mentions the inner status 7.
	if !strings.Contains(body, `"exit_code":`) || strings.Contains(body, `"exit_code":0`) {
		t.Errorf("expected non-zero exit_code, got %s", body)
	}
	if !strings.Contains(body, "status 7") && !strings.Contains(body, "intentionally broken") {
		t.Errorf("expected stderr from broken block in error, got %s", body)
	}
	if len(cons.Settled) != 1 || cons.Settled[0] != "ack" {
		t.Errorf("block failure must still ack the job, got %v", cons.Settled)
	}
}
