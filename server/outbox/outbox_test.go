package outbox

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	err = db.Exec(`CREATE TABLE runs (
		id          TEXT PRIMARY KEY,
		owner_id    TEXT NOT NULL DEFAULT '',
		yaml        TEXT NOT NULL DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'queued',
		error       TEXT,
		started_at  DATETIME,
		finished_at DATETIME,
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error
	if err != nil {
		t.Fatalf("create test table: %v", err)
	}
	return db
}

func insertRun(t *testing.T, db *gorm.DB, id, status string) {
	t.Helper()
	err := db.Exec(
		`INSERT INTO runs (id, owner_id, yaml, status) VALUES (?, 'user-1', 'name: test', ?)`,
		id, status,
	).Error
	if err != nil {
		t.Fatalf("insert run %s: %v", id, err)
	}
}

func runStatus(t *testing.T, db *gorm.DB, id string) string {
	t.Helper()
	var s string
	if err := db.Raw("SELECT status FROM runs WHERE id = ?", id).Scan(&s).Error; err != nil {
		t.Fatalf("read status for %s: %v", id, err)
	}
	return s
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// --- FetchQueued ---

func TestFetchQueued_ReturnsOnlyQueued(t *testing.T) {
	db := newTestDB(t)
	insertRun(t, db, "aaa", "queued")
	insertRun(t, db, "bbb", "queued")
	insertRun(t, db, "ccc", "running")
	insertRun(t, db, "ddd", "succeeded")

	rows, err := FetchQueued(context.Background(), db, 10)
	if err != nil {
		t.Fatalf("FetchQueued: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 queued rows, got %d", len(rows))
	}
	ids := map[string]bool{rows[0].ID: true, rows[1].ID: true}
	if !ids["aaa"] || !ids["bbb"] {
		t.Errorf("unexpected row ids: %v", rows)
	}
}

func TestFetchQueued_RespectsLimit(t *testing.T) {
	db := newTestDB(t)
	insertRun(t, db, "a1", "queued")
	insertRun(t, db, "a2", "queued")
	insertRun(t, db, "a3", "queued")

	rows, err := FetchQueued(context.Background(), db, 2)
	if err != nil {
		t.Fatalf("FetchQueued: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 with limit=2, got %d", len(rows))
	}
}

func TestFetchQueued_PopulatesFields(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec(
		`INSERT INTO runs (id, owner_id, yaml, status) VALUES ('r1', 'u1', 'pipeline: yaml', 'queued')`,
	).Error; err != nil {
		t.Fatal(err)
	}

	rows, err := FetchQueued(context.Background(), db, 10)
	if err != nil {
		t.Fatalf("FetchQueued: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.ID != "r1" || r.OwnerID != "u1" || r.YAML != "pipeline: yaml" {
		t.Errorf("unexpected row contents: %+v", r)
	}
}

// --- MarkRunning ---

func TestMarkRunning_TransitionsToRunning(t *testing.T) {
	db := newTestDB(t)
	insertRun(t, db, "aaa", "queued")

	if err := MarkRunning(context.Background(), db, "aaa"); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}
	if s := runStatus(t, db, "aaa"); s != "running" {
		t.Errorf("status after MarkRunning: got %q, want 'running'", s)
	}
	// Should no longer appear in FetchQueued.
	rows, _ := FetchQueued(context.Background(), db, 10)
	for _, r := range rows {
		if r.ID == "aaa" {
			t.Error("run still appears as queued after MarkRunning")
		}
	}
}

func TestMarkRunning_Idempotency(t *testing.T) {
	db := newTestDB(t)
	insertRun(t, db, "aaa", "queued")

	results := make([]error, 2)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = MarkRunning(context.Background(), db, "aaa")
		}(i)
	}
	wg.Wait()

	nils, claims := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			nils++
		case errors.Is(err, ErrAlreadyClaimed):
			claims++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if nils != 1 || claims != 1 {
		t.Errorf("expected exactly one nil and one ErrAlreadyClaimed; got nils=%d claims=%d", nils, claims)
	}
}

func TestMarkRunning_AlreadyRunning_ReturnsClaimed(t *testing.T) {
	db := newTestDB(t)
	insertRun(t, db, "aaa", "running") // already running

	err := MarkRunning(context.Background(), db, "aaa")
	if !errors.Is(err, ErrAlreadyClaimed) {
		t.Errorf("expected ErrAlreadyClaimed, got %v", err)
	}
}

// --- MarkFailed ---

func TestMarkFailed_TransitionsToFailed(t *testing.T) {
	db := newTestDB(t)
	insertRun(t, db, "aaa", "running")

	if err := MarkFailed(context.Background(), db, "aaa", "engine exploded"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if s := runStatus(t, db, "aaa"); s != "failed" {
		t.Errorf("status after MarkFailed: got %q, want 'failed'", s)
	}
	rows, _ := FetchQueued(context.Background(), db, 10)
	if len(rows) != 0 {
		t.Errorf("expected 0 queued rows after MarkFailed, got %d", len(rows))
	}
}

// --- poller (poll) ---

func TestPoller_SubmitsAndMarksRunning(t *testing.T) {
	db := newTestDB(t)
	insertRun(t, db, "aaa", "queued")

	var submitted []string
	submit := func(_ context.Context, run QueuedRun) error {
		submitted = append(submitted, run.ID)
		return nil
	}

	poll(context.Background(), db, submit, silentLogger())

	if len(submitted) != 1 || submitted[0] != "aaa" {
		t.Errorf("expected [aaa] submitted, got %v", submitted)
	}
	if s := runStatus(t, db, "aaa"); s != "running" {
		t.Errorf("run not marked running after poll: %q", s)
	}
}

func TestPoller_MarksFailedOnSubmitError(t *testing.T) {
	db := newTestDB(t)
	insertRun(t, db, "aaa", "queued")

	submit := func(_ context.Context, run QueuedRun) error {
		return errors.New("submission failed")
	}

	poll(context.Background(), db, submit, silentLogger())

	if s := runStatus(t, db, "aaa"); s != "failed" {
		t.Errorf("expected failed after submit error, got %q", s)
	}
}

func TestPoller_SkipsAlreadyClaimed(t *testing.T) {
	db := newTestDB(t)
	insertRun(t, db, "aaa", "running") // already claimed

	submitCalled := false
	submit := func(_ context.Context, run QueuedRun) error {
		submitCalled = true
		return nil
	}

	poll(context.Background(), db, submit, silentLogger())

	if submitCalled {
		t.Error("submit called for an already-claimed (running) run")
	}
}

func TestPoller_ProcessesMultipleRuns(t *testing.T) {
	db := newTestDB(t)
	insertRun(t, db, "r1", "queued")
	insertRun(t, db, "r2", "queued")
	insertRun(t, db, "r3", "queued")

	var mu sync.Mutex
	var submitted []string
	submit := func(_ context.Context, run QueuedRun) error {
		mu.Lock()
		submitted = append(submitted, run.ID)
		mu.Unlock()
		return nil
	}

	poll(context.Background(), db, submit, silentLogger())

	if len(submitted) != 3 {
		t.Errorf("expected 3 submitted, got %d: %v", len(submitted), submitted)
	}
}
