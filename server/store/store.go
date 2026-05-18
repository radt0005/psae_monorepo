package store

import (
	"context"

	"github.com/google/uuid"
)

// Store is the persistence contract used by the scheduler engine and
// HTTP API.  Implementations include *PgStore (PostgreSQL via GORM) and
// *MemStore (in-memory, for tests).
type Store interface {
	// InsertPipeline writes a brand-new pipeline header and stamps it
	// SubmittedAt=now and Status=running.  It returns ErrAlreadyExists
	// if a pipeline with the same ID is already persisted.
	InsertPipeline(ctx context.Context, p PipelineRecord) error

	// LoadPipeline returns the persisted header by ID, ErrNotFound if
	// no such row exists.
	LoadPipeline(ctx context.Context, id uuid.UUID) (PipelineRecord, error)

	// ListPipelines returns a paginated list of summaries matching the
	// filter, newest first.
	ListPipelines(ctx context.Context, filter ListFilter) ([]PipelineSummary, error)

	// UpdatePipelineStatus moves the pipeline to a new lifecycle state
	// and stamps CompletedAt when status is a terminal one.  Returns
	// ErrNotFound if the pipeline is unknown.
	UpdatePipelineStatus(ctx context.Context, id uuid.UUID, status PipelineStatus) error

	// UpsertInvocation idempotently inserts or updates a row keyed by
	// InvocationRecord.ID.  Crucially: a row already in a terminal
	// status (complete/error) is NOT overwritten by a subsequent call,
	// matching the "first result wins" guarantee from
	// worker.md §Result reporting.
	UpsertInvocation(ctx context.Context, rec InvocationRecord) error

	// LoadInvocations returns every invocation row for the pipeline,
	// in insertion order.
	LoadInvocations(ctx context.Context, pipelineID uuid.UUID) ([]InvocationRecord, error)

	// LoadInvocation returns a single invocation row by ID.
	LoadInvocation(ctx context.Context, invocationID string) (InvocationRecord, error)

	// LoadActivePipelinesForRestart returns every pipeline still in
	// status "running" together with its full invocation history.
	// Called once at engine startup to rebuild in-memory state.
	LoadActivePipelinesForRestart(ctx context.Context) ([]ActivePipeline, error)

	// AppendEvent writes one row to the pipeline_events audit log.
	AppendEvent(ctx context.Context, evt PipelineEvent) error

	// Close releases any underlying resources (database connections, etc.).
	Close() error
}

// ErrAlreadyExists is returned by InsertPipeline when a pipeline with the
// same ID already exists.  Engine treats this as a non-retryable
// validation error.
type errAlreadyExists struct{}

func (errAlreadyExists) Error() string { return "pipeline already exists" }

// ErrAlreadyExists is the sentinel value compared via errors.Is.
var ErrAlreadyExists error = errAlreadyExists{}
