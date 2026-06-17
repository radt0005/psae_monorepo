// Package mirror maintains the web_ui metadata read-mirror (registry.md §10):
// the registry pushes block metadata into the system's Postgres `blocks` table
// so the editor's block picker can browse without querying the registry on
// every request. The registry remains the source of truth; the mirror is a
// best-effort, rebuildable replica.
package mirror

import (
	"encoding/json"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"spade_registry/internal/store"
)

// Mirror is the contract the state machine drives on transitions.
type Mirror interface {
	// UpsertVersion publishes a version's blocks to the browse mirror.
	UpsertVersion(v *store.Version, collectionName string, blocks []store.BlockMeta) error
	// RemoveVersion hides a version's blocks from browse (deprecate/yank/recall).
	RemoveVersion(blocks []store.BlockMeta) error
}

// NoopMirror discards all updates (used in tests and when mirroring is off).
type NoopMirror struct{}

func (NoopMirror) UpsertVersion(*store.Version, string, []store.BlockMeta) error { return nil }
func (NoopMirror) RemoveVersion([]store.BlockMeta) error                         { return nil }

// blockRow maps the web_ui-owned `blocks` table (drizzle schema in
// web_ui/server/db/schema/blocks.ts). The registry writes only the columns
// that table already has; see the schema-limitation note in the README.
type blockRow struct {
	ID        string         `gorm:"column:id;primaryKey"`
	Name      string         `gorm:"column:name"`
	Label     string         `gorm:"column:label"`
	Version   string         `gorm:"column:version"`
	Manifest  datatypes.JSON `gorm:"column:manifest"`
	CreatedAt time.Time      `gorm:"column:created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
}

func (blockRow) TableName() string { return "blocks" }

// PostgresMirror upserts into the shared `blocks` table.
type PostgresMirror struct {
	db *gorm.DB
}

// NewPostgres wraps a *gorm.DB pointed at the shared application database.
func NewPostgres(db *gorm.DB) *PostgresMirror { return &PostgresMirror{db: db} }

func (m *PostgresMirror) UpsertVersion(v *store.Version, collectionName string, blocks []store.BlockMeta) error {
	if len(blocks) == 0 {
		return nil
	}
	rows := make([]blockRow, 0, len(blocks))
	now := time.Now()
	for _, b := range blocks {
		manifest := buildManifestJSON(collectionName, v.Version, b)
		rows = append(rows, blockRow{
			ID:        b.BlockID,
			Name:      b.Name,
			Label:     labelFor(b),
			Version:   v.Version,
			Manifest:  manifest,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return m.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "label", "version", "manifest", "updated_at"}),
	}).Create(&rows).Error
}

func (m *PostgresMirror) RemoveVersion(blocks []store.BlockMeta) error {
	if len(blocks) == 0 {
		return nil
	}
	ids := make([]string, len(blocks))
	for i, b := range blocks {
		ids[i] = b.BlockID
	}
	return m.db.Where("id IN ?", ids).Delete(&blockRow{}).Error
}

func labelFor(b store.BlockMeta) string {
	if b.Description != "" {
		return b.Description
	}
	return b.Name
}

// buildManifestJSON assembles the manifest jsonb the editor consumes, folding
// in the collection/version/state context the bare `blocks` columns lack.
func buildManifestJSON(collection, version string, b store.BlockMeta) datatypes.JSON {
	var inputs, outputs any
	_ = json.Unmarshal(b.Inputs, &inputs)
	_ = json.Unmarshal(b.Outputs, &outputs)
	m := map[string]any{
		"id":          b.BlockID,
		"name":        b.Name,
		"collection":  collection,
		"version":     version,
		"kind":        b.Kind,
		"network":     b.Network,
		"description": b.Description,
		"entrypoint":  b.Entrypoint,
		"inputs":      inputs,
		"outputs":     outputs,
	}
	raw, _ := json.Marshal(m)
	return raw
}
