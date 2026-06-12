// Package outbox implements the Postgres outbox consumer for the Spade
// scheduler.  The web UI writes pipeline runs as 'queued' rows in the
// shared PostgreSQL runs table; this package polls for those rows and
// hands them to the engine for execution.
//
// spec/worker.md §Communication: "the web UI ↔ scheduler path uses a
// Postgres outbox."
package outbox

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// ErrAlreadyClaimed is returned by MarkRunning when another poller instance
// claimed the same run first.  The optimistic lock is the WHERE clause
// "status = 'queued'" on the UPDATE — zero rows affected means another
// caller won the race.
var ErrAlreadyClaimed = errors.New("run already claimed by another poller")

// QueuedRun carries the fields the scheduler needs to execute a submitted
// pipeline.  The ID doubles as the pipeline ID in the scheduler's own store.
type QueuedRun struct {
	ID      string // UUIDv7 run id, shared with the web UI's runs table
	OwnerID string // web UI user id, forwarded as the scheduler submitter
	YAML    string // resolved (UUID-only) pipeline YAML snapshot
}

// FetchQueued returns up to limit runs from the web UI's runs table where
// status = 'queued', ordered oldest-first so the earliest submissions run
// first.
func FetchQueued(ctx context.Context, db *gorm.DB, limit int) ([]QueuedRun, error) {
	type row struct {
		ID      string `gorm:"column:id"`
		OwnerID string `gorm:"column:owner_id"`
		YAML    string `gorm:"column:yaml"`
	}
	var rows []row
	err := db.WithContext(ctx).
		Table("runs").
		Select("id, owner_id, yaml").
		Where("status = ?", "queued").
		Order("created_at ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]QueuedRun, len(rows))
	for i, r := range rows {
		out[i] = QueuedRun{ID: r.ID, OwnerID: r.OwnerID, YAML: r.YAML}
	}
	return out, nil
}

// MarkRunning atomically moves the run from 'queued' to 'running'.
// Returns ErrAlreadyClaimed if another poller instance got there first.
func MarkRunning(ctx context.Context, db *gorm.DB, id string) error {
	now := time.Now().UTC()
	result := db.WithContext(ctx).
		Table("runs").
		Where("id = ? AND status = ?", id, "queued").
		Updates(map[string]any{
			"status":     "running",
			"started_at": now,
			"updated_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAlreadyClaimed
	}
	return nil
}

// MarkFailed moves the run to 'failed' with a diagnostic error message.
// Called when the scheduler cannot submit the pipeline (e.g. bad YAML).
func MarkFailed(ctx context.Context, db *gorm.DB, id, errMsg string) error {
	now := time.Now().UTC()
	return db.WithContext(ctx).
		Table("runs").
		Where("id = ?", id).
		Updates(map[string]any{
			"status":      "failed",
			"error":       errMsg,
			"finished_at": now,
			"updated_at":  now,
		}).Error
}
