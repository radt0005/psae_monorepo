package store

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ErrNotFound is returned by lookups when no matching row exists.
var ErrNotFound = errors.New("store: not found")

// Store wraps a *gorm.DB with registry-specific queries.
type Store struct {
	db *gorm.DB
}

// Open opens a Postgres-backed store and migrates the registry-owned tables.
func Open(dsn string) (*Store, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening postgres: %w", err)
	}
	return newStore(db)
}

// OpenSQLite opens a file-backed SQLite store (used in tests and trivial local
// runs) and migrates the registry-owned tables.
func OpenSQLite(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}
	return newStore(db)
}

// OpenSQLiteMem opens an in-memory SQLite store (fast unit tests).
func OpenSQLiteMem() (*Store, error) {
	// A shared cache keeps the single connection's schema visible across the
	// pool for the lifetime of the process-unique DSN.
	return OpenSQLite("file::memory:?cache=shared")
}

func newStore(db *gorm.DB) (*Store, error) {
	if err := db.AutoMigrate(AllModels()...); err != nil {
		return nil, fmt.Errorf("automigrate: %w", err)
	}
	return &Store{db: db}, nil
}

// DB exposes the underlying *gorm.DB for callers that need raw access
// (e.g. the mirror writer targeting web_ui-owned tables).
func (s *Store) DB() *gorm.DB { return s.db }

// ---- Collections & versions -------------------------------------------------

// GetCollectionByName returns a collection by name, or ErrNotFound.
func (s *Store) GetCollectionByName(name string) (*Collection, error) {
	var c Collection
	err := s.db.Where("name = ?", name).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// EnsureCollection returns the existing collection by name or creates a new one
// owned by ownerUserID. The bool reports whether a new collection was created.
func (s *Store) EnsureCollection(name, ownerUserID, language string) (*Collection, bool, error) {
	c, err := s.GetCollectionByName(name)
	if err == nil {
		return c, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, false, err
	}
	c = &Collection{
		ID:          NewID(),
		Name:        name,
		OwnerUserID: ownerUserID,
		Language:    language,
	}
	if err := s.db.Create(c).Error; err != nil {
		return nil, false, err
	}
	return c, true, nil
}

// GetVersion returns a version by (collection name, version), or ErrNotFound.
func (s *Store) GetVersion(collectionName, version string) (*Version, error) {
	c, err := s.GetCollectionByName(collectionName)
	if err != nil {
		return nil, err
	}
	var v Version
	err = s.db.Where("collection_id = ? AND version = ?", c.ID, version).First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// GetVersionByID returns a version by primary key, or ErrNotFound.
func (s *Store) GetVersionByID(id string) (*Version, error) {
	var v Version
	err := s.db.Where("id = ?", id).First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// CreateVersion inserts a new version row.
func (s *Store) CreateVersion(v *Version) error {
	if v.ID == "" {
		v.ID = NewID()
	}
	return s.db.Create(v).Error
}

// SetVersionState updates only the state (and optional error message) of a
// version. The transition-rule and authz checks live in internal/state.
func (s *Store) SetVersionState(versionID string, to State, errMsg string) error {
	return s.db.Model(&Version{}).
		Where("id = ?", versionID).
		Updates(map[string]any{"state": to, "error": errMsg, "updated_at": time.Now()}).
		Error
}

// ListCollections returns all collections.
func (s *Store) ListCollections() ([]Collection, error) {
	var cs []Collection
	if err := s.db.Order("name").Find(&cs).Error; err != nil {
		return nil, err
	}
	return cs, nil
}

// ListVersions returns all versions of a collection ordered by creation time.
func (s *Store) ListVersions(collectionName string) ([]Version, error) {
	c, err := s.GetCollectionByName(collectionName)
	if err != nil {
		return nil, err
	}
	var vs []Version
	if err := s.db.Where("collection_id = ?", c.ID).Order("created_at").Find(&vs).Error; err != nil {
		return nil, err
	}
	return vs, nil
}

// ---- Artifacts & block metadata --------------------------------------------

// GetArtifact looks up a built artifact by collection/version/platform/arch.
func (s *Store) GetArtifact(collectionName, version, platform, arch string) (*Artifact, *Version, error) {
	v, err := s.GetVersion(collectionName, version)
	if err != nil {
		return nil, nil, err
	}
	var a Artifact
	err = s.db.Where("version_id = ? AND platform = ? AND arch = ?", v.ID, platform, arch).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, v, ErrNotFound
	}
	if err != nil {
		return nil, v, err
	}
	return &a, v, nil
}

// CreateArtifact inserts an artifact row.
func (s *Store) CreateArtifact(a *Artifact) error {
	if a.ID == "" {
		a.ID = NewID()
	}
	return s.db.Create(a).Error
}

// ReplaceBlockMeta deletes any existing block metadata for a version and
// inserts the provided set (idempotent on re-screen/re-build).
func (s *Store) ReplaceBlockMeta(versionID string, blocks []BlockMeta) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("version_id = ?", versionID).Delete(&BlockMeta{}).Error; err != nil {
			return err
		}
		for i := range blocks {
			if blocks[i].ID == "" {
				blocks[i].ID = NewID()
			}
			blocks[i].VersionID = versionID
		}
		if len(blocks) == 0 {
			return nil
		}
		return tx.Create(&blocks).Error
	})
}

// ListBlockMeta returns the block metadata for a version.
func (s *Store) ListBlockMeta(versionID string) ([]BlockMeta, error) {
	var bs []BlockMeta
	if err := s.db.Where("version_id = ?", versionID).Find(&bs).Error; err != nil {
		return nil, err
	}
	return bs, nil
}

// ---- Screening --------------------------------------------------------------

// CreateScreeningResult records a screener outcome.
func (s *Store) CreateScreeningResult(r *ScreeningResult) error {
	if r.ID == "" {
		r.ID = NewID()
	}
	return s.db.Create(r).Error
}

// ListScreeningResults returns screening results for a version.
func (s *Store) ListScreeningResults(versionID string) ([]ScreeningResult, error) {
	var rs []ScreeningResult
	if err := s.db.Where("version_id = ?", versionID).Order("created_at").Find(&rs).Error; err != nil {
		return nil, err
	}
	return rs, nil
}

// ---- Build jobs -------------------------------------------------------------

// CreateBuildJob inserts a queued build job.
func (s *Store) CreateBuildJob(j *BuildJob) error {
	if j.ID == "" {
		j.ID = NewID()
	}
	return s.db.Create(j).Error
}

// GetBuildJob returns a build job by id, or ErrNotFound.
func (s *Store) GetBuildJob(id string) (*BuildJob, error) {
	var j BuildJob
	err := s.db.Where("id = ?", id).First(&j).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// ClaimNextBuildJob atomically claims the oldest queued job, transitioning it to
// BuildClaimed. Returns (nil, ErrNotFound) when the queue is empty.
func (s *Store) ClaimNextBuildJob() (*BuildJob, error) {
	var claimed *BuildJob
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var j BuildJob
		q := tx.Where("state = ?", BuildQueued).Order("created_at")
		// Postgres path: skip-locked avoids two dispatchers grabbing one job.
		if tx.Dialector.Name() == "postgres" {
			q = q.Clauses(lockingForUpdateSkipLocked())
		}
		err := q.First(&j).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		now := time.Now()
		if err := tx.Model(&j).Updates(map[string]any{
			"state": BuildClaimed, "claimed_at": now, "attempts": j.Attempts + 1, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		j.State = BuildClaimed
		j.ClaimedAt = &now
		j.Attempts++
		claimed = &j
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

// ListStuckBuildJobs returns claimed/running jobs whose last update is older
// than staleBefore — orphans left behind by a dispatcher that died between
// claiming a job and recording a terminal state.
func (s *Store) ListStuckBuildJobs(staleBefore time.Time) ([]BuildJob, error) {
	var js []BuildJob
	err := s.db.
		Where("state IN ? AND updated_at < ?", []BuildJobState{BuildClaimed, BuildRunning}, staleBefore).
		Order("created_at").Find(&js).Error
	if err != nil {
		return nil, err
	}
	return js, nil
}

// RequeueBuildJob returns a job to the queue, clearing the claim and the old
// builder token so the next dispatch mints a fresh one. Attempts are preserved
// (they increment at claim time), which is how the reaper's retry cap counts.
func (s *Store) RequeueBuildJob(id string) error {
	return s.db.Model(&BuildJob{}).Where("id = ?", id).Updates(map[string]any{
		"state": BuildQueued, "token_hash": "", "container_id": "",
		"claimed_at": nil, "updated_at": time.Now(),
	}).Error
}

// SetBuildJobState updates a build job's state (and optional container id).
func (s *Store) SetBuildJobState(id string, to BuildJobState, containerID string) error {
	updates := map[string]any{"state": to, "updated_at": time.Now()}
	if containerID != "" {
		updates["container_id"] = containerID
	}
	return s.db.Model(&BuildJob{}).Where("id = ?", id).Updates(updates).Error
}

// SetBuildJobToken records the hash of the per-job builder token (minted at
// dispatch time, not at publish time).
func (s *Store) SetBuildJobToken(id, tokenHash string) error {
	return s.db.Model(&BuildJob{}).Where("id = ?", id).
		Update("token_hash", tokenHash).Error
}

// SetBuildJobLogs records the logs object key for a build job.
func (s *Store) SetBuildJobLogs(id, logsKey string) error {
	return s.db.Model(&BuildJob{}).Where("id = ?", id).
		Update("logs_key", logsKey).Error
}
