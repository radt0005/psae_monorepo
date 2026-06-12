package outbox

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

// Run polls the web UI's runs table at the given interval, calling submit for
// each newly-queued run.  submit is responsible for handing the pipeline off
// to the engine.  Run blocks until ctx is cancelled.
func Run(ctx context.Context, db *gorm.DB, submit func(ctx context.Context, run QueuedRun) error, interval time.Duration, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poll(ctx, db, submit, logger)
		}
	}
}

// poll is one sweep of the outbox: fetch up to 10 queued runs, claim each one,
// and call submit.  Package-private so tests can invoke it directly without
// fighting the ticker.
func poll(ctx context.Context, db *gorm.DB, submit func(ctx context.Context, run QueuedRun) error, logger *slog.Logger) {
	runs, err := FetchQueued(ctx, db, 10)
	if err != nil {
		logger.Error("outbox: fetch queued runs failed", "err", err)
		return
	}
	for _, run := range runs {
		if err := MarkRunning(ctx, db, run.ID); err != nil {
			if errors.Is(err, ErrAlreadyClaimed) {
				continue
			}
			logger.Error("outbox: mark running failed", "run_id", run.ID, "err", err)
			continue
		}
		if err := submit(ctx, run); err != nil {
			logger.Error("outbox: submit failed", "run_id", run.ID, "err", err)
			if mfErr := MarkFailed(ctx, db, run.ID, err.Error()); mfErr != nil {
				logger.Error("outbox: mark failed failed", "run_id", run.ID, "err", mfErr)
			}
		}
	}
}
