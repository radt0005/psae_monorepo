// Package auth provides the registry's three identity checks (registry.md §7):
// developers (Better Auth session token), workers (rotated service token), and
// builders (per-job token). These are pure verifiers; HTTP middleware wiring
// lives in internal/api.
package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"

	"spade_registry/internal/store"
)

// ErrUnauthenticated is returned when a credential is missing or invalid.
var ErrUnauthenticated = errors.New("auth: unauthenticated")

// HashToken returns the hex sha256 of a token. Service and builder tokens are
// stored hashed so a DB read cannot recover a usable credential.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Developer identifies an authenticated human developer.
type Developer struct {
	UserID string
}

// DeveloperVerifier validates a developer bearer token.
type DeveloperVerifier interface {
	Verify(token string) (Developer, error)
}

// SessionTableVerifier validates a Better Auth session token directly against
// the shared `session` table (token match + not expired). It is pluggable so an
// HTTP-introspection verifier can replace it without touching callers.
type SessionTableVerifier struct {
	db *gorm.DB
}

// NewSessionVerifier wraps the shared application *gorm.DB.
func NewSessionVerifier(db *gorm.DB) *SessionTableVerifier {
	return &SessionTableVerifier{db: db}
}

// sessionRow maps the Better Auth `session` table (web_ui schema/sessions.ts).
type sessionRow struct {
	ID        string    `gorm:"column:id"`
	UserID    string    `gorm:"column:userId"`
	Token     string    `gorm:"column:token"`
	ExpiresAt time.Time `gorm:"column:expiresAt"`
}

func (sessionRow) TableName() string { return "session" }

func (v *SessionTableVerifier) Verify(token string) (Developer, error) {
	if token == "" {
		return Developer{}, ErrUnauthenticated
	}
	var row sessionRow
	err := v.db.Table("session").Where("token = ?", token).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Developer{}, ErrUnauthenticated
	}
	if err != nil {
		return Developer{}, err
	}
	if time.Now().After(row.ExpiresAt) {
		return Developer{}, ErrUnauthenticated
	}
	return Developer{UserID: row.UserID}, nil
}

// WorkerAuth validates a worker's rotated read-only service token.
type WorkerAuth struct {
	st *store.Store
}

// NewWorkerAuth wraps the registry store.
func NewWorkerAuth(st *store.Store) *WorkerAuth { return &WorkerAuth{st: st} }

// Worker identifies an authenticated worker.
type Worker struct {
	ID   string
	Name string
}

// Verify checks the presented service token against active service tokens.
func (w *WorkerAuth) Verify(token string) (Worker, error) {
	if token == "" {
		return Worker{}, ErrUnauthenticated
	}
	row, err := w.st.ActiveServiceTokenByHash(HashToken(token))
	if errors.Is(err, store.ErrNotFound) {
		return Worker{}, ErrUnauthenticated
	}
	if err != nil {
		return Worker{}, err
	}
	return Worker{ID: row.ID, Name: row.Name}, nil
}

// BuilderAuth validates a per-job builder token against the build job.
type BuilderAuth struct {
	st *store.Store
}

// NewBuilderAuth wraps the registry store.
func NewBuilderAuth(st *store.Store) *BuilderAuth { return &BuilderAuth{st: st} }

// Verify checks token against the build job's stored hash and rejects jobs that
// are already past the running state.
func (b *BuilderAuth) Verify(jobID, token string) (*store.BuildJob, error) {
	if token == "" {
		return nil, ErrUnauthenticated
	}
	job, err := b.st.GetBuildJob(jobID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	if job.TokenHash == "" || job.TokenHash != HashToken(token) {
		return nil, ErrUnauthenticated
	}
	switch job.State {
	case store.BuildSucceeded, store.BuildFailed:
		return nil, ErrUnauthenticated // job is closed
	}
	return job, nil
}
