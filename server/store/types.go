// Package store is the durable persistence layer for the spade-scheduler.
//
// scheduler.md §State Management says PostgreSQL is the source of truth
// for pipeline DAGs and invocation result history.  This package
// implements that source of truth via the Store interface, with two
// implementations:
//
//   - *PgStore — production PostgreSQL via GORM.  Configured at startup
//     from SPADE_DATABASE_URL.
//   - *MemStore — in-memory implementation used by unit tests so the
//     engine and HTTP API can be exercised without a live database.
//
// The Store interface intentionally exposes only the methods the engine
// and API layers call; the GORM models stay package-private to keep the
// abstraction tight.
package store

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// PipelineStatus is the lifecycle status of a pipeline.
type PipelineStatus string

const (
	PipelineRunning   PipelineStatus = "running"
	PipelineComplete  PipelineStatus = "complete"
	PipelineFailed    PipelineStatus = "failed"
	PipelineCancelled PipelineStatus = "cancelled"
)

// InvocationStatus is the lifecycle status of a single block invocation.
type InvocationStatus string

const (
	InvocationPending    InvocationStatus = "pending"
	InvocationReady      InvocationStatus = "ready"
	InvocationDispatched InvocationStatus = "dispatched"
	InvocationComplete   InvocationStatus = "complete"
	InvocationError      InvocationStatus = "error"
	InvocationMap        InvocationStatus = "map"
	InvocationReduce     InvocationStatus = "reduce"
)

// PipelineRecord is the persisted form of a pipeline.
type PipelineRecord struct {
	ID              uuid.UUID `gorm:"primaryKey;type:uuid"`
	Name            string    `gorm:"index"`
	Version         string
	Description     string
	YAML            string         `gorm:"type:text"`
	Status          PipelineStatus `gorm:"index"`
	SubmittedAt     time.Time
	CompletedAt     *time.Time
	SubmitterUserID string
}

// TableName returns the PostgreSQL table name for PipelineRecord.
//
// Prefixed with `scheduler_` to avoid colliding with the web UI's own
// `pipelines` table: both services share one database (hosting.md §6.1),
// but the web UI's `pipelines` (user-authored saved pipelines, owner_id
// NOT NULL) is a different concept from the scheduler's runtime DAG state.
func (PipelineRecord) TableName() string { return "scheduler_pipelines" }

// InvocationRecord is the persisted form of a single block invocation.
// ID is the invocation-ID string form (`<UUID>` or `<UUID>.<index>`)
// which is the natural idempotency key per worker.md §Result reporting.
type InvocationRecord struct {
	ID               string    `gorm:"primaryKey;type:text"`
	PipelineID       uuid.UUID `gorm:"index;type:uuid"`
	BlockID          uuid.UUID `gorm:"index;type:uuid"`
	BlockName        string
	MapIndex         *int
	Status           InvocationStatus `gorm:"index"`
	DispatchedAt     *time.Time
	CompletedAt      *time.Time
	ExitCode         int
	LogsPath         string
	ErrorMessage     string `gorm:"type:text"`
	OutputHashesJSON string `gorm:"type:text"`
	ExpansionJSON    string `gorm:"type:text"`
}

// TableName returns the PostgreSQL table name for InvocationRecord.
// Prefixed with `scheduler_` for the same reason as PipelineRecord.
func (InvocationRecord) TableName() string { return "scheduler_invocations" }

// EventType labels persisted PipelineEvents.
type EventType string

const (
	EventSubmitted         EventType = "submitted"
	EventBlockDispatched   EventType = "block_dispatched"
	EventBlockCompleted    EventType = "block_completed"
	EventBlockFailed       EventType = "block_failed"
	EventPipelineCompleted EventType = "pipeline_completed"
	EventPipelineCancelled EventType = "pipeline_cancelled"
	EventPipelineFailed    EventType = "pipeline_failed"
)

// PipelineEvent is an audit-log row.  Every state transition writes one.
type PipelineEvent struct {
	ID           uint      `gorm:"primaryKey"`
	PipelineID   uuid.UUID `gorm:"index;type:uuid"`
	InvocationID string    `gorm:"index"`
	EventType    EventType `gorm:"index"`
	PayloadJSON  string    `gorm:"type:text"`
	CreatedAt    time.Time `gorm:"index"`
}

// TableName returns the PostgreSQL table name for PipelineEvent.
// Prefixed with `scheduler_` for the same reason as PipelineRecord.
func (PipelineEvent) TableName() string { return "scheduler_pipeline_events" }

// ErrNotFound is returned when a record is missing.
var ErrNotFound = errors.New("record not found")

// PipelineSummary is a compact view returned by ListPipelines.
type PipelineSummary struct {
	ID          uuid.UUID
	Name        string
	Version     string
	Status      PipelineStatus
	SubmittedAt time.Time
	CompletedAt *time.Time
}

// ListFilter narrows ListPipelines.  Empty status matches all.
type ListFilter struct {
	Status    PipelineStatus
	Submitter string
	Limit     int
	Offset    int
}

// ActivePipeline is the bundle returned by LoadActivePipelinesForRestart.
// It contains everything the engine needs to call Rehydrate on startup:
// the raw YAML (so the pipeline can be parsed via core.LoadPipeline),
// the parsed pipeline record header, and the full invocation history.
type ActivePipeline struct {
	Pipeline    PipelineRecord
	Invocations []InvocationRecord
}
