package mirror

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"spade_registry/internal/store"
)

// newBlocksDB stands up a SQLite DB with the web_ui-owned `blocks` table shape
// so the mirror writer can be exercised without a real Postgres.
func newBlocksDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/blocks.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.Table("blocks").AutoMigrate(&blockRow{}))
	return db
}

func TestPostgresMirrorUpsertAndRemove(t *testing.T) {
	db := newBlocksDB(t)
	m := NewPostgres(db)

	v := &store.Version{Version: "1.0.0"}
	blocks := []store.BlockMeta{
		{BlockID: "gdal.rasterize", Name: "rasterize", Description: "Rasterize vectors", Kind: "standard"},
		{BlockID: "gdal.reproject", Name: "reproject"},
	}

	require.NoError(t, m.UpsertVersion(v, "gdal", blocks))

	var count int64
	require.NoError(t, db.Table("blocks").Count(&count).Error)
	require.Equal(t, int64(2), count)

	var row blockRow
	require.NoError(t, db.Table("blocks").Where("id = ?", "gdal.rasterize").First(&row).Error)
	require.Equal(t, "rasterize", row.Name)
	require.Equal(t, "Rasterize vectors", row.Label)
	require.Contains(t, string(row.Manifest), "\"collection\":\"gdal\"")

	// Re-upsert updates rather than duplicating.
	blocks[0].Description = "Updated"
	require.NoError(t, m.UpsertVersion(v, "gdal", blocks))
	require.NoError(t, db.Table("blocks").Count(&count).Error)
	require.Equal(t, int64(2), count)
	require.NoError(t, db.Table("blocks").Where("id = ?", "gdal.rasterize").First(&row).Error)
	require.Equal(t, "Updated", row.Label)

	// Remove hides them from browse.
	require.NoError(t, m.RemoveVersion(blocks))
	require.NoError(t, db.Table("blocks").Count(&count).Error)
	require.Equal(t, int64(0), count)
}

func TestNoopMirror(t *testing.T) {
	var m Mirror = NoopMirror{}
	require.NoError(t, m.UpsertVersion(&store.Version{}, "c", []store.BlockMeta{{BlockID: "a"}}))
	require.NoError(t, m.RemoveVersion([]store.BlockMeta{{BlockID: "a"}}))
}
