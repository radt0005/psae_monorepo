package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemStore is the in-memory Store implementation used in tests.
//
// It is goroutine-safe.  The implementation favors simplicity over
// performance: linear scans through all rows are fine for the workload
// expected from unit tests.
type MemStore struct {
	mu          sync.Mutex
	pipelines   map[uuid.UUID]PipelineRecord
	invocations map[string]InvocationRecord
	events      []PipelineEvent
}

// NewMemStore returns an empty in-memory Store.
func NewMemStore() *MemStore {
	return &MemStore{
		pipelines:   make(map[uuid.UUID]PipelineRecord),
		invocations: make(map[string]InvocationRecord),
	}
}

// InsertPipeline inserts a brand-new pipeline row.  Returns
// ErrAlreadyExists if the ID is already known.
func (m *MemStore) InsertPipeline(ctx context.Context, p PipelineRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.pipelines[p.ID]; exists {
		return ErrAlreadyExists
	}
	if p.SubmittedAt.IsZero() {
		p.SubmittedAt = time.Now().UTC()
	}
	if p.Status == "" {
		p.Status = PipelineRunning
	}
	m.pipelines[p.ID] = p
	return nil
}

// LoadPipeline returns the persisted header.
func (m *MemStore) LoadPipeline(ctx context.Context, id uuid.UUID) (PipelineRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.pipelines[id]
	if !ok {
		return PipelineRecord{}, ErrNotFound
	}
	return rec, nil
}

// ListPipelines returns matching pipelines newest first.
func (m *MemStore) ListPipelines(ctx context.Context, filter ListFilter) ([]PipelineSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []PipelineSummary
	for _, rec := range m.pipelines {
		if filter.Status != "" && rec.Status != filter.Status {
			continue
		}
		if filter.Submitter != "" && rec.SubmitterUserID != filter.Submitter {
			continue
		}
		out = append(out, PipelineSummary{
			ID:          rec.ID,
			Name:        rec.Name,
			Version:     rec.Version,
			Status:      rec.Status,
			SubmittedAt: rec.SubmittedAt,
			CompletedAt: rec.CompletedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SubmittedAt.After(out[j].SubmittedAt) })
	if filter.Offset > 0 {
		if filter.Offset >= len(out) {
			return nil, nil
		}
		out = out[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(out) {
		out = out[:filter.Limit]
	}
	return out, nil
}

// UpdatePipelineStatus stamps a new lifecycle state.
func (m *MemStore) UpdatePipelineStatus(ctx context.Context, id uuid.UUID, status PipelineStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.pipelines[id]
	if !ok {
		return ErrNotFound
	}
	rec.Status = status
	switch status {
	case PipelineComplete, PipelineFailed, PipelineCancelled:
		t := time.Now().UTC()
		rec.CompletedAt = &t
	}
	m.pipelines[id] = rec
	return nil
}

// UpsertInvocation idempotently inserts or updates.  A terminal status
// already on the row prevents overwrite — the first-result-wins guarantee.
func (m *MemStore) UpsertInvocation(ctx context.Context, rec InvocationRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, exists := m.invocations[rec.ID]
	if exists {
		// Terminal statuses block overwrite.
		if existing.Status == InvocationComplete || existing.Status == InvocationError {
			return nil
		}
		// Merge: preserve non-empty fields when caller passes zero.
		if rec.Status == "" {
			rec.Status = existing.Status
		}
		if rec.DispatchedAt == nil {
			rec.DispatchedAt = existing.DispatchedAt
		}
		if rec.CompletedAt == nil {
			rec.CompletedAt = existing.CompletedAt
		}
		if rec.LogsPath == "" {
			rec.LogsPath = existing.LogsPath
		}
		if rec.ErrorMessage == "" {
			rec.ErrorMessage = existing.ErrorMessage
		}
		if rec.OutputHashesJSON == "" {
			rec.OutputHashesJSON = existing.OutputHashesJSON
		}
		if rec.ExpansionJSON == "" {
			rec.ExpansionJSON = existing.ExpansionJSON
		}
	}
	m.invocations[rec.ID] = rec
	return nil
}

// LoadInvocations returns all rows for the pipeline in stable order.
func (m *MemStore) LoadInvocations(ctx context.Context, pipelineID uuid.UUID) ([]InvocationRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []InvocationRecord
	for _, rec := range m.invocations {
		if rec.PipelineID == pipelineID {
			out = append(out, rec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// LoadInvocation returns a single invocation row.
func (m *MemStore) LoadInvocation(ctx context.Context, invocationID string) (InvocationRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.invocations[invocationID]
	if !ok {
		return InvocationRecord{}, ErrNotFound
	}
	return rec, nil
}

// LoadActivePipelinesForRestart returns running pipelines with their
// invocation history.
func (m *MemStore) LoadActivePipelinesForRestart(ctx context.Context) ([]ActivePipeline, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ActivePipeline
	for _, rec := range m.pipelines {
		if rec.Status != PipelineRunning {
			continue
		}
		var invs []InvocationRecord
		for _, inv := range m.invocations {
			if inv.PipelineID == rec.ID {
				invs = append(invs, inv)
			}
		}
		sort.Slice(invs, func(i, j int) bool { return invs[i].ID < invs[j].ID })
		out = append(out, ActivePipeline{Pipeline: rec, Invocations: invs})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Pipeline.SubmittedAt.Before(out[j].Pipeline.SubmittedAt) })
	return out, nil
}

// AppendEvent records an audit-log entry.
func (m *MemStore) AppendEvent(ctx context.Context, evt PipelineEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	evt.ID = uint(len(m.events) + 1)
	if evt.CreatedAt.IsZero() {
		evt.CreatedAt = time.Now().UTC()
	}
	m.events = append(m.events, evt)
	return nil
}

// Events returns a copy of all audit-log rows.  Test-only helper.
func (m *MemStore) Events() []PipelineEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]PipelineEvent, len(m.events))
	copy(out, m.events)
	return out
}

// Close is a no-op for the in-memory store.
func (m *MemStore) Close() error { return nil }

// Compile-time interface check.
var _ Store = (*MemStore)(nil)
