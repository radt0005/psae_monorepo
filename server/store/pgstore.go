package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

// PgStore is the GORM-backed Store.  The same implementation works
// against PostgreSQL in production and SQLite in tests; see
// NewPgStore vs NewSQLiteStore.
type PgStore struct {
	db *gorm.DB
}

// NewPgStore opens a PostgreSQL connection at dsn, auto-migrates the
// schema, and returns a ready-to-use Store.
//
// scheduler.md §State Management requires PostgreSQL as the source of
// truth; the binary refuses to start if the connection fails.
func NewPgStore(dsn string) (*PgStore, error) {
	if dsn == "" {
		return nil, errors.New("empty database DSN")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening postgres connection: %w", err)
	}
	s := &PgStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// NewSQLiteStore opens a SQLite-backed store at the given path.  This
// path is for tests and local development only; production uses
// NewPgStore.  Use ":memory:" for an ephemeral test store.
func NewSQLiteStore(path string) (*PgStore, error) {
	if path == "" {
		path = ":memory:"
	}
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening sqlite store: %w", err)
	}
	s := &PgStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PgStore) migrate() error {
	return s.db.AutoMigrate(
		&PipelineRecord{},
		&InvocationRecord{},
		&PipelineEvent{},
	)
}

// InsertPipeline writes a new pipeline header.  Returns ErrAlreadyExists
// on duplicate primary key.
func (s *PgStore) InsertPipeline(ctx context.Context, p PipelineRecord) error {
	if p.SubmittedAt.IsZero() {
		p.SubmittedAt = time.Now().UTC()
	}
	if p.Status == "" {
		p.Status = PipelineRunning
	}
	res := s.db.WithContext(ctx).Create(&p)
	if res.Error != nil {
		// GORM does not normalize "duplicate key" errors across drivers;
		// fall back to an Existence check on conflict-shaped errors.
		var existing PipelineRecord
		if err := s.db.WithContext(ctx).Where("id = ?", p.ID).First(&existing).Error; err == nil {
			return ErrAlreadyExists
		}
		return fmt.Errorf("inserting pipeline %s: %w", p.ID, res.Error)
	}
	return nil
}

// LoadPipeline fetches a pipeline header by ID.
func (s *PgStore) LoadPipeline(ctx context.Context, id uuid.UUID) (PipelineRecord, error) {
	var rec PipelineRecord
	res := s.db.WithContext(ctx).Where("id = ?", id).First(&rec)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return PipelineRecord{}, ErrNotFound
	}
	return rec, res.Error
}

// ListPipelines returns matching pipelines newest first.
func (s *PgStore) ListPipelines(ctx context.Context, filter ListFilter) ([]PipelineSummary, error) {
	q := s.db.WithContext(ctx).Model(&PipelineRecord{})
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.Submitter != "" {
		q = q.Where("submitter_user_id = ?", filter.Submitter)
	}
	q = q.Order("submitted_at DESC")
	if filter.Limit > 0 {
		q = q.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		q = q.Offset(filter.Offset)
	}
	var recs []PipelineRecord
	if err := q.Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]PipelineSummary, 0, len(recs))
	for _, r := range recs {
		out = append(out, PipelineSummary{
			ID:          r.ID,
			Name:        r.Name,
			Version:     r.Version,
			Status:      r.Status,
			SubmittedAt: r.SubmittedAt,
			CompletedAt: r.CompletedAt,
		})
	}
	return out, nil
}

// UpdatePipelineStatus moves the pipeline to a new lifecycle status.
func (s *PgStore) UpdatePipelineStatus(ctx context.Context, id uuid.UUID, status PipelineStatus) error {
	updates := map[string]any{"status": status}
	if status == PipelineComplete || status == PipelineFailed || status == PipelineCancelled {
		t := time.Now().UTC()
		updates["completed_at"] = &t
	}
	res := s.db.WithContext(ctx).Model(&PipelineRecord{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpsertInvocation idempotently inserts or updates a row.  Terminal
// statuses prevent later writes from overwriting via a WHERE clause on
// the ON CONFLICT DO UPDATE — the first-result-wins guarantee.
func (s *PgStore) UpsertInvocation(ctx context.Context, rec InvocationRecord) error {
	// Use ON CONFLICT DO UPDATE keyed on ID.  GORM's clause.OnConflict
	// generates the correct dialect-specific SQL.  The "Where" clause
	// blocks overwriting an already-terminal row.
	conflict := clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"pipeline_id", "block_id", "block_name", "map_indices",
			"status", "dispatched_at", "completed_at",
			"exit_code", "logs_path", "error_message",
			"output_hashes_json", "expansion_json",
		}),
		Where: clause.Where{
			Exprs: []clause.Expression{
				clause.AndConditions{
					Exprs: []clause.Expression{
						clause.Neq{Column: clause.Column{Table: (InvocationRecord{}).TableName(), Name: "status"}, Value: string(InvocationComplete)},
						clause.Neq{Column: clause.Column{Table: (InvocationRecord{}).TableName(), Name: "status"}, Value: string(InvocationError)},
					},
				},
			},
		},
	}
	res := s.db.WithContext(ctx).Clauses(conflict).Create(&rec)
	return res.Error
}

// LoadInvocations returns every invocation row for the pipeline.
func (s *PgStore) LoadInvocations(ctx context.Context, pipelineID uuid.UUID) ([]InvocationRecord, error) {
	var out []InvocationRecord
	res := s.db.WithContext(ctx).Where("pipeline_id = ?", pipelineID).Order("id ASC").Find(&out)
	return out, res.Error
}

// LoadInvocation fetches a single invocation row.
func (s *PgStore) LoadInvocation(ctx context.Context, invocationID string) (InvocationRecord, error) {
	var rec InvocationRecord
	res := s.db.WithContext(ctx).Where("id = ?", invocationID).First(&rec)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return InvocationRecord{}, ErrNotFound
	}
	return rec, res.Error
}

// LoadActivePipelinesForRestart returns running pipelines together with
// their invocation history.  Called once at engine startup.
func (s *PgStore) LoadActivePipelinesForRestart(ctx context.Context) ([]ActivePipeline, error) {
	var pipes []PipelineRecord
	if err := s.db.WithContext(ctx).Where("status = ?", PipelineRunning).Find(&pipes).Error; err != nil {
		return nil, err
	}
	out := make([]ActivePipeline, 0, len(pipes))
	for _, p := range pipes {
		invs, err := s.LoadInvocations(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, ActivePipeline{Pipeline: p, Invocations: invs})
	}
	return out, nil
}

// AppendEvent persists an audit-log row.
func (s *PgStore) AppendEvent(ctx context.Context, evt PipelineEvent) error {
	if evt.CreatedAt.IsZero() {
		evt.CreatedAt = time.Now().UTC()
	}
	return s.db.WithContext(ctx).Create(&evt).Error
}

// Close releases the underlying database connection.
func (s *PgStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Compile-time interface check.
var _ Store = (*PgStore)(nil)
