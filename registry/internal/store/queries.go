package store

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// lockingForUpdateSkipLocked returns a SELECT ... FOR UPDATE SKIP LOCKED clause
// (Postgres only) so concurrent dispatchers never claim the same build job.
func lockingForUpdateSkipLocked() clause.Locking {
	return clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}
}

// ---- Signing keys -----------------------------------------------------------

// CreateSigningKey inserts a signing key.
func (s *Store) CreateSigningKey(k *SigningKey) error {
	if k.ID == "" {
		k.ID = NewID()
	}
	return s.db.Create(k).Error
}

// ActiveSigningKey returns the single active signing key, or ErrNotFound.
func (s *Store) ActiveSigningKey() (*SigningKey, error) {
	var k SigningKey
	err := s.db.Where("active = ?", true).First(&k).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// ListedSigningKeys returns all keys served by /pubkeys.
func (s *Store) ListedSigningKeys() ([]SigningKey, error) {
	var ks []SigningKey
	if err := s.db.Where("listed = ?", true).Order("created_at").Find(&ks).Error; err != nil {
		return nil, err
	}
	return ks, nil
}

// DeactivateSigningKeys clears the active flag on all keys (used before
// promoting a new active key during rotation).
func (s *Store) DeactivateSigningKeys() error {
	return s.db.Model(&SigningKey{}).Where("active = ?", true).
		Update("active", false).Error
}

// RetireSigningKey unlists a key and stamps its retirement time.
func (s *Store) RetireSigningKey(id string) error {
	now := time.Now()
	return s.db.Model(&SigningKey{}).Where("id = ?", id).
		Updates(map[string]any{"active": false, "listed": false, "retired_at": now}).Error
}

// ---- Service tokens ---------------------------------------------------------

// CreateServiceToken inserts a worker service token (TokenHash already set).
func (s *Store) CreateServiceToken(t *ServiceToken) error {
	if t.ID == "" {
		t.ID = NewID()
	}
	return s.db.Create(t).Error
}

// ActiveServiceTokenByHash returns the active service token matching tokenHash,
// or ErrNotFound. Touches LastUsedAt as a side effect.
func (s *Store) ActiveServiceTokenByHash(tokenHash string) (*ServiceToken, error) {
	var t ServiceToken
	err := s.db.Where("token_hash = ? AND active = ?", tokenHash, true).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	now := time.Now()
	s.db.Model(&ServiceToken{}).Where("id = ?", t.ID).Update("last_used_at", now)
	return &t, nil
}

// DeactivateServiceToken marks a service token inactive.
func (s *Store) DeactivateServiceToken(id string) error {
	return s.db.Model(&ServiceToken{}).Where("id = ?", id).
		Update("active", false).Error
}

// ---- Audit ------------------------------------------------------------------

// CreateAuditEntry appends an audit record.
func (s *Store) CreateAuditEntry(e *AuditEntry) error {
	if e.ID == "" {
		e.ID = NewID()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	return s.db.Create(e).Error
}

// ListAuditEntries returns audit entries, most recent first, up to limit.
func (s *Store) ListAuditEntries(limit int) ([]AuditEntry, error) {
	var es []AuditEntry
	q := s.db.Order("created_at desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&es).Error; err != nil {
		return nil, err
	}
	return es, nil
}
