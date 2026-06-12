package engine

import (
	"encoding/json"
	"fmt"

	"core"

	"gorm.io/gorm"
)

// PgManifestProvider is the production ManifestProvider.  It reads block
// manifests from the registry metadata mirror in PostgreSQL — the `blocks`
// table the web UI also serves to its editor (registry.md §10,
// web_ui/server/db/schema/blocks.ts).  Until the cloud registry exists, that
// table is populated by the local seed-blocks service (seed/seed.sh).
//
// The `manifest` column is jsonb holding the full BlockManifest; core's
// manifest types carry json tags so it unmarshals directly.
type PgManifestProvider struct {
	db *gorm.DB
}

// NewPgManifestProvider returns a provider backed by the given GORM
// connection (typically the same UI-database handle the outbox poller uses).
func NewPgManifestProvider(db *gorm.DB) *PgManifestProvider {
	return &PgManifestProvider{db: db}
}

// blockRow is the subset of the `blocks` table this provider reads.
type blockRow struct {
	Name     string `gorm:"column:name"`
	Manifest []byte `gorm:"column:manifest"`
}

// Lookup resolves a single block name to its manifest.  The version arg is
// accepted for interface compatibility but not yet used: the mirror holds one
// row per block name (see the unique constraint on blocks.name).
func (p *PgManifestProvider) Lookup(name, _ string) (core.BlockManifest, error) {
	var row blockRow
	err := p.db.Table("blocks").
		Select("name, manifest").
		Where("name = ?", name).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return core.BlockManifest{}, ErrManifestNotFound
		}
		return core.BlockManifest{}, fmt.Errorf("looking up manifest %q: %w", name, err)
	}
	return decodeManifest(row.Manifest)
}

// All snapshots every mirrored manifest keyed by block name.  The engine
// type-asserts for this method to build the map it validates pipelines
// against (engine.go manifestMap); without it, validation degrades to an
// empty set and every block reads as an unknown type.
func (p *PgManifestProvider) All() map[string]core.BlockManifest {
	var rows []blockRow
	if err := p.db.Table("blocks").Select("name, manifest").Find(&rows).Error; err != nil {
		// All() has no error channel; an empty map is the safe degraded
		// result (validation will report unknown block types, which is
		// preferable to dispatching against a partial set).
		return map[string]core.BlockManifest{}
	}
	out := make(map[string]core.BlockManifest, len(rows))
	for _, row := range rows {
		m, err := decodeManifest(row.Manifest)
		if err != nil {
			continue
		}
		out[row.Name] = m
	}
	return out
}

func decodeManifest(raw []byte) (core.BlockManifest, error) {
	var m core.BlockManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return core.BlockManifest{}, fmt.Errorf("decoding manifest jsonb: %w", err)
	}
	if m.Kind == "" {
		m.Kind = core.BlockKindStandard
	}
	return m, nil
}
