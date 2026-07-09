// Package store persists envelope-encrypted secret ciphertext and the audit
// log in the managed PostgreSQL database (spec/secrets.md §5.1, §5.4). The same
// GORM-backed implementation runs against PostgreSQL in production and SQLite
// in tests, mirroring the pattern in server/store.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

// ErrNotFound is returned when a secret does not exist for an owner.
var ErrNotFound = errors.New("secret not found")

// Secret is the stored, envelope-encrypted form of a user secret. Only
// ciphertext lives here; the KEK never touches the database (hosting.md §6.1).
type Secret struct {
	OwnerID    string `gorm:"primaryKey"`
	Name       string `gorm:"primaryKey"`
	Ciphertext []byte `gorm:"column:ciphertext"`
	ValueNonce []byte `gorm:"column:value_nonce"`
	WrappedDEK []byte `gorm:"column:wrapped_dek"`
	DEKNonce   []byte `gorm:"column:dek_nonce"`
	KEKID      string `gorm:"column:kek_id"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AuditRecord is one entry in the secrets audit log (spec/secrets.md §5.4).
type AuditRecord struct {
	ID           uint `gorm:"primaryKey"`
	OwnerID      string
	InvocationID string
	SecretNames  string // comma-separated names touched by the action
	Actor        string
	Action       string // "set" | "delete" | "resolve"
	At           time.Time
}

// Store is the persistence surface the KMS API depends on.
type Store interface {
	Upsert(ctx context.Context, s Secret) error
	Get(ctx context.Context, owner, name string) (Secret, error)
	ListNames(ctx context.Context, owner string) ([]string, error)
	Delete(ctx context.Context, owner, name string) error
	Audit(ctx context.Context, a AuditRecord) error
}

// GormStore is the GORM-backed Store.
type GormStore struct{ db *gorm.DB }

// NewPgStore opens a PostgreSQL connection, migrates, and returns a Store.
func NewPgStore(dsn string) (*GormStore, error) {
	if dsn == "" {
		return nil, errors.New("empty database DSN")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening postgres connection: %w", err)
	}
	return newGormStore(db)
}

// NewSQLiteStore opens a SQLite-backed store (tests and local dev only). Use
// ":memory:" for an ephemeral store.
func NewSQLiteStore(path string) (*GormStore, error) {
	if path == "" {
		path = ":memory:"
	}
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening sqlite store: %w", err)
	}
	return newGormStore(db)
}

func newGormStore(db *gorm.DB) (*GormStore, error) {
	s := &GormStore{db: db}
	if err := db.AutoMigrate(&Secret{}, &AuditRecord{}); err != nil {
		return nil, fmt.Errorf("migrating schema: %w", err)
	}
	return s, nil
}

func (s *GormStore) Upsert(ctx context.Context, sec Secret) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "owner_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"ciphertext", "value_nonce", "wrapped_dek", "dek_nonce", "kek_id", "updated_at",
		}),
	}).Create(&sec).Error
}

func (s *GormStore) Get(ctx context.Context, owner, name string) (Secret, error) {
	var sec Secret
	err := s.db.WithContext(ctx).
		Where("owner_id = ? AND name = ?", owner, name).First(&sec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Secret{}, ErrNotFound
	}
	return sec, err
}

func (s *GormStore) ListNames(ctx context.Context, owner string) ([]string, error) {
	var names []string
	err := s.db.WithContext(ctx).Model(&Secret{}).
		Where("owner_id = ?", owner).Order("name").Pluck("name", &names).Error
	return names, err
}

func (s *GormStore) Delete(ctx context.Context, owner, name string) error {
	res := s.db.WithContext(ctx).
		Where("owner_id = ? AND name = ?", owner, name).Delete(&Secret{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *GormStore) Audit(ctx context.Context, a AuditRecord) error {
	if a.At.IsZero() {
		a.At = time.Now()
	}
	return s.db.WithContext(ctx).Create(&a).Error
}
