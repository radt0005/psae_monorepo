// Package store defines the registry's authoritative persistence layer: the
// GORM models for collections, versions, artifacts, build jobs, screening
// results, signing keys, service tokens, and the audit log, plus the queries
// over them. Postgres is used in production; SQLite is used in tests.
//
// These are the registry's *own* tables. The web_ui-owned `blocks` and
// `session` tables are never auto-migrated here (see internal/mirror and
// internal/auth, which read/write them as a guest).
package store

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// State is a collection-version lifecycle state (registry.md §3).
type State string

const (
	StateSubmitted  State = "submitted"
	StateScreening  State = "screening"
	StateScreened   State = "screened"
	StateBuilding   State = "building"
	StateAvailable  State = "available"
	StateDeprecated State = "deprecated"
	StateYanked     State = "yanked"
	StateRecalled   State = "recalled"
	StateFailed     State = "failed"
)

// Valid reports whether s is a known state.
func (s State) Valid() bool {
	switch s {
	case StateSubmitted, StateScreening, StateScreened, StateBuilding,
		StateAvailable, StateDeprecated, StateYanked, StateRecalled, StateFailed:
		return true
	}
	return false
}

// BuildJobState is the state of a queued build (the registry's internal queue).
type BuildJobState string

const (
	BuildQueued    BuildJobState = "queued"
	BuildClaimed   BuildJobState = "claimed"
	BuildRunning   BuildJobState = "running"
	BuildSucceeded BuildJobState = "succeeded"
	BuildFailed    BuildJobState = "failed"
)

// Collection is a named, owned group of blocks sharing a language (blocks.md §2).
type Collection struct {
	ID          string `gorm:"primaryKey"`
	Name        string `gorm:"uniqueIndex;not null"`
	OwnerUserID string `gorm:"index;not null"`
	Language    string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Versions []Version `gorm:"foreignKey:CollectionID"`
}

// Version is a single published version of a collection and its lifecycle state.
type Version struct {
	ID                string `gorm:"primaryKey"`
	CollectionID      string `gorm:"index:idx_collection_version,unique;not null"`
	Version           string `gorm:"index:idx_collection_version,unique;not null"`
	State             State  `gorm:"index;not null"`
	RepoURL           string
	CommitSHA         string
	SubmittedByUserID string
	Error             string
	CreatedAt         time.Time
	UpdatedAt         time.Time

	Collection Collection  `gorm:"foreignKey:CollectionID"`
	Artifacts  []Artifact  `gorm:"foreignKey:VersionID"`
	Blocks     []BlockMeta `gorm:"foreignKey:VersionID"`
}

// Artifact is a built, signed tarball for one platform/arch of a version.
// Key format: <collection>/<version>/<platform>/<arch>.tar.gz (+ .sig).
type Artifact struct {
	ID           string `gorm:"primaryKey"`
	VersionID    string `gorm:"index:idx_version_platform_arch,unique;not null"`
	Platform     string `gorm:"index:idx_version_platform_arch,unique;not null"`
	Arch         string `gorm:"index:idx_version_platform_arch,unique;not null"`
	ContentHash  string `gorm:"not null"` // sha256 hex over the tarball bytes
	ArtifactKey  string `gorm:"not null"`
	SigKey       string `gorm:"not null"`
	SigningKeyID string
	SizeBytes    int64
	CreatedAt    time.Time
}

// BlockMeta is per-block manifest metadata captured at build completion; it
// feeds the web_ui metadata mirror (registry.md §10).
type BlockMeta struct {
	ID          string `gorm:"primaryKey"`
	VersionID   string `gorm:"index;not null"`
	BlockID     string `gorm:"not null"` // e.g. "gdal.rasterize"
	Name        string `gorm:"not null"`
	Kind        string
	Network     bool
	Description string
	Entrypoint  string
	Inputs      datatypes.JSON
	Outputs     datatypes.JSON
}

// ScreeningResult records the outcome of one screener run against a version.
type ScreeningResult struct {
	ID              string `gorm:"primaryKey"`
	VersionID       string `gorm:"index;not null"`
	ScreenerName    string
	ScreenerVersion string
	Passed          bool
	Details         datatypes.JSON
	CreatedAt       time.Time
}

// BuildJob is an entry in the registry's internal build queue. The dispatcher
// claims jobs and launches an ephemeral per-language build container; the
// builder reports back over the HTTP API authenticating with TokenHash.
type BuildJob struct {
	ID          string        `gorm:"primaryKey"`
	VersionID   string        `gorm:"index;not null"`
	Language    string        `gorm:"index"`
	State       BuildJobState `gorm:"index;not null"`
	TokenHash   string        // sha256 of the per-job builder token
	ContainerID string
	Attempts    int
	LogsKey     string
	ClaimedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SigningKey is an ed25519 keypair. Exactly one row is Active (used to sign);
// all Listed rows are served by /pubkeys (registry.md §6.1 key rotation).
type SigningKey struct {
	ID         string `gorm:"primaryKey"`
	PublicKey  string `gorm:"not null"` // base64 std
	PrivateKey string // base64 std; dev-only storage (see internal/sign)
	Active     bool   `gorm:"index"`
	Listed     bool   `gorm:"index"`
	CreatedAt  time.Time
	RetiredAt  *time.Time
}

// ServiceToken is a worker's rotated read-only service token (registry.md §7.2).
type ServiceToken struct {
	ID            string `gorm:"primaryKey"`
	Name          string `gorm:"index"` // worker id
	TokenHash     string `gorm:"uniqueIndex;not null"`
	Scope         string
	Active        bool `gorm:"index"`
	RotatedFromID string
	LastUsedAt    *time.Time
	CreatedAt     time.Time
}

// AuditEntry is an append-only audit record (registry.md §7.3).
type AuditEntry struct {
	ID         string `gorm:"primaryKey"`
	EventType  string `gorm:"index"` // publish|transition|fetch|screening|build
	ActorID    string
	ActorType  string // developer|worker|operator|system
	Collection string
	Version    string
	FromState  string
	ToState    string
	Reason     string
	Detail     datatypes.JSON
	CreatedAt  time.Time
}

// NewID returns a fresh UUIDv4 string for use as a primary key.
func NewID() string { return uuid.NewString() }

// AllModels is the list AutoMigrate operates on (registry-owned tables only).
func AllModels() []any {
	return []any{
		&Collection{}, &Version{}, &Artifact{}, &BlockMeta{},
		&ScreeningResult{}, &BuildJob{}, &SigningKey{}, &ServiceToken{},
		&AuditEntry{},
	}
}
