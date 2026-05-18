package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// storeImpls returns the implementations to exercise via every shared test.
func storeImpls(t *testing.T) []struct {
	name string
	open func(t *testing.T) Store
} {
	t.Helper()
	return []struct {
		name string
		open func(t *testing.T) Store
	}{
		{name: "mem", open: func(t *testing.T) Store { return NewMemStore() }},
		{name: "sqlite", open: func(t *testing.T) Store {
			s, err := NewSQLiteStore(":memory:")
			if err != nil {
				t.Fatalf("opening sqlite: %v", err)
			}
			return s
		}},
	}
}

func TestInsertAndLoadPipeline(t *testing.T) {
	for _, impl := range storeImpls(t) {
		t.Run(impl.name, func(t *testing.T) {
			s := impl.open(t)
			defer s.Close()

			id := uuid.Must(uuid.NewV7())
			p := PipelineRecord{
				ID:      id,
				Name:    "tp",
				Version: "1.0",
				YAML:    "name: tp",
				Status:  PipelineRunning,
			}
			ctx := context.Background()
			if err := s.InsertPipeline(ctx, p); err != nil {
				t.Fatalf("InsertPipeline: %v", err)
			}
			got, err := s.LoadPipeline(ctx, id)
			if err != nil {
				t.Fatalf("LoadPipeline: %v", err)
			}
			if got.ID != id || got.Name != "tp" {
				t.Fatalf("round trip changed record: %+v", got)
			}
		})
	}
}

func TestInsertPipelineDuplicate(t *testing.T) {
	for _, impl := range storeImpls(t) {
		t.Run(impl.name, func(t *testing.T) {
			s := impl.open(t)
			defer s.Close()
			ctx := context.Background()
			p := PipelineRecord{ID: uuid.Must(uuid.NewV7()), Name: "a", Status: PipelineRunning}
			if err := s.InsertPipeline(ctx, p); err != nil {
				t.Fatalf("first insert: %v", err)
			}
			err := s.InsertPipeline(ctx, p)
			if !errors.Is(err, ErrAlreadyExists) {
				t.Fatalf("expected ErrAlreadyExists, got %v", err)
			}
		})
	}
}

func TestLoadPipelineNotFound(t *testing.T) {
	for _, impl := range storeImpls(t) {
		t.Run(impl.name, func(t *testing.T) {
			s := impl.open(t)
			defer s.Close()
			_, err := s.LoadPipeline(context.Background(), uuid.Must(uuid.NewV7()))
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("expected ErrNotFound, got %v", err)
			}
		})
	}
}

func TestUpdatePipelineStatusStampsCompletedAt(t *testing.T) {
	for _, impl := range storeImpls(t) {
		t.Run(impl.name, func(t *testing.T) {
			s := impl.open(t)
			defer s.Close()
			ctx := context.Background()
			id := uuid.Must(uuid.NewV7())
			if err := s.InsertPipeline(ctx, PipelineRecord{ID: id, Name: "a", Status: PipelineRunning}); err != nil {
				t.Fatal(err)
			}
			if err := s.UpdatePipelineStatus(ctx, id, PipelineComplete); err != nil {
				t.Fatal(err)
			}
			rec, err := s.LoadPipeline(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if rec.Status != PipelineComplete {
				t.Errorf("status: got %s want complete", rec.Status)
			}
			if rec.CompletedAt == nil {
				t.Errorf("expected CompletedAt set")
			}
		})
	}
}

func TestUpsertInvocationFirstResultWins(t *testing.T) {
	for _, impl := range storeImpls(t) {
		t.Run(impl.name, func(t *testing.T) {
			s := impl.open(t)
			defer s.Close()
			ctx := context.Background()
			pid := uuid.Must(uuid.NewV7())
			invID := uuid.Must(uuid.NewV7()).String()

			// First write: complete with exit 0.
			first := InvocationRecord{
				ID:         invID,
				PipelineID: pid,
				BlockName:  "b",
				Status:     InvocationComplete,
				ExitCode:   0,
				LogsPath:   "first-logs",
			}
			if err := s.UpsertInvocation(ctx, first); err != nil {
				t.Fatalf("first upsert: %v", err)
			}
			// Second write: try to overwrite with exit 1.
			second := first
			second.ExitCode = 1
			second.LogsPath = "second-logs"
			if err := s.UpsertInvocation(ctx, second); err != nil {
				t.Fatalf("second upsert: %v", err)
			}
			got, err := s.LoadInvocation(ctx, invID)
			if err != nil {
				t.Fatal(err)
			}
			if got.ExitCode != 0 || got.LogsPath != "first-logs" {
				t.Errorf("first-result-wins violated: %+v", got)
			}
		})
	}
}

func TestUpsertInvocationProgressiveStatus(t *testing.T) {
	for _, impl := range storeImpls(t) {
		t.Run(impl.name, func(t *testing.T) {
			s := impl.open(t)
			defer s.Close()
			ctx := context.Background()
			pid := uuid.Must(uuid.NewV7())
			invID := uuid.Must(uuid.NewV7()).String()

			// Pending → Dispatched → Complete.
			now := time.Now().UTC()
			steps := []InvocationRecord{
				{ID: invID, PipelineID: pid, Status: InvocationPending, BlockName: "b"},
				{ID: invID, PipelineID: pid, Status: InvocationDispatched, BlockName: "b", DispatchedAt: &now},
				{ID: invID, PipelineID: pid, Status: InvocationComplete, BlockName: "b", ExitCode: 0, CompletedAt: &now},
			}
			for _, step := range steps {
				if err := s.UpsertInvocation(ctx, step); err != nil {
					t.Fatalf("step %s: %v", step.Status, err)
				}
			}
			rec, err := s.LoadInvocation(ctx, invID)
			if err != nil {
				t.Fatal(err)
			}
			if rec.Status != InvocationComplete {
				t.Fatalf("final status: %s", rec.Status)
			}
		})
	}
}

func TestLoadActivePipelinesForRestart(t *testing.T) {
	for _, impl := range storeImpls(t) {
		t.Run(impl.name, func(t *testing.T) {
			s := impl.open(t)
			defer s.Close()
			ctx := context.Background()

			active := PipelineRecord{ID: uuid.Must(uuid.NewV7()), Name: "active", Status: PipelineRunning, YAML: "x"}
			done := PipelineRecord{ID: uuid.Must(uuid.NewV7()), Name: "done", Status: PipelineComplete, YAML: "y"}
			for _, p := range []PipelineRecord{active, done} {
				if err := s.InsertPipeline(ctx, p); err != nil {
					t.Fatal(err)
				}
			}
			// Persist one invocation for the active pipeline.
			now := time.Now().UTC()
			rec := InvocationRecord{
				ID: uuid.Must(uuid.NewV7()).String(), PipelineID: active.ID,
				BlockName: "a", Status: InvocationDispatched, DispatchedAt: &now,
			}
			if err := s.UpsertInvocation(ctx, rec); err != nil {
				t.Fatal(err)
			}
			recovered, err := s.LoadActivePipelinesForRestart(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(recovered) != 1 {
				t.Fatalf("expected 1 active pipeline, got %d", len(recovered))
			}
			if recovered[0].Pipeline.ID != active.ID {
				t.Errorf("recovered wrong pipeline")
			}
			if len(recovered[0].Invocations) != 1 {
				t.Errorf("expected 1 invocation, got %d", len(recovered[0].Invocations))
			}
		})
	}
}

func TestAppendAndListEventsMemStore(t *testing.T) {
	s := NewMemStore()
	ctx := context.Background()
	pid := uuid.Must(uuid.NewV7())
	for _, evt := range []EventType{EventSubmitted, EventBlockDispatched, EventPipelineCompleted} {
		if err := s.AppendEvent(ctx, PipelineEvent{PipelineID: pid, EventType: evt}); err != nil {
			t.Fatal(err)
		}
	}
	evts := s.Events()
	if len(evts) != 3 {
		t.Fatalf("expected 3 events, got %d", len(evts))
	}
	for i, want := range []EventType{EventSubmitted, EventBlockDispatched, EventPipelineCompleted} {
		if evts[i].EventType != want {
			t.Errorf("evt[%d]: got %s want %s", i, evts[i].EventType, want)
		}
	}
}

func TestListPipelinesPagination(t *testing.T) {
	s := NewMemStore()
	ctx := context.Background()
	base := time.Now().UTC()
	for i := 0; i < 5; i++ {
		p := PipelineRecord{ID: uuid.Must(uuid.NewV7()), Name: "p", Status: PipelineRunning, SubmittedAt: base.Add(time.Duration(i) * time.Second)}
		if err := s.InsertPipeline(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	results, err := s.ListPipelines(ctx, ListFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Newest-first ordering: earliest submitted is base+4s.
	if !results[0].SubmittedAt.After(results[1].SubmittedAt) {
		t.Errorf("expected newest first, got %v then %v", results[0].SubmittedAt, results[1].SubmittedAt)
	}
}
