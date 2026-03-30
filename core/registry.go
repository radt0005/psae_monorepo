package core

import (
	"fmt"
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// BlockRegistry manages the SQLite block registry database.
type BlockRegistry struct {
	db *gorm.DB
}

// OpenRegistry opens (or creates) the SQLite database at the given path with GORM,
// enables WAL mode, sets file permissions to 0600, and auto-migrates the schema.
func OpenRegistry(dbPath string) (*BlockRegistry, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening registry database: %w", err)
	}

	// Enable WAL mode for concurrent access
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("getting sql.DB: %w", err)
	}
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}

	// Set file permissions to 0600
	if err := os.Chmod(dbPath, 0600); err != nil {
		return nil, fmt.Errorf("setting database permissions: %w", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&BlockRegistryEntry{}); err != nil {
		return nil, fmt.Errorf("migrating schema: %w", err)
	}

	return &BlockRegistry{db: db}, nil
}

// RegisterBlock inserts or updates a block entry in the registry.
func (r *BlockRegistry) RegisterBlock(entry BlockRegistryEntry) error {
	result := r.db.Where("block_id = ? AND collection_version = ?", entry.BlockID, entry.CollectionVersion).
		Assign(entry).
		FirstOrCreate(&entry)
	return result.Error
}

// LookupBlock queries the registry by block name and optional version.
// If version is empty, returns the latest version.
func (r *BlockRegistry) LookupBlock(name string, version string) (*BlockRegistryEntry, error) {
	var entry BlockRegistryEntry
	q := r.db.Where("block_id = ?", name)
	if version != "" {
		q = q.Where("collection_version = ?", version)
	} else {
		q = q.Order("collection_version DESC")
	}
	result := q.First(&entry)
	if result.Error != nil {
		return nil, result.Error
	}
	return &entry, nil
}

// ListBlocks returns all registered blocks.
func (r *BlockRegistry) ListBlocks() ([]BlockRegistryEntry, error) {
	var entries []BlockRegistryEntry
	result := r.db.Find(&entries)
	return entries, result.Error
}

// DeleteCollection removes all blocks for a given collection and version.
func (r *BlockRegistry) DeleteCollection(name string, version string) error {
	result := r.db.Where("collection_name = ? AND collection_version = ?", name, version).
		Delete(&BlockRegistryEntry{})
	return result.Error
}

// RebuildFromFilesystem scans a blocks directory, re-reads all block manifests,
// recomputes content hashes, and repopulates the database.
func (r *BlockRegistry) RebuildFromFilesystem(blocksDir string) error {
	// Clear existing entries
	if err := r.db.Where("1 = 1").Delete(&BlockRegistryEntry{}).Error; err != nil {
		return fmt.Errorf("clearing registry: %w", err)
	}

	// Scan for installed collections
	collections, err := os.ReadDir(blocksDir)
	if err != nil {
		return fmt.Errorf("reading blocks directory %s: %w", blocksDir, err)
	}

	for _, collection := range collections {
		if !collection.IsDir() {
			continue
		}
		collectionPath := fmt.Sprintf("%s/%s", blocksDir, collection.Name())
		versions, err := os.ReadDir(collectionPath)
		if err != nil {
			continue
		}

		for _, version := range versions {
			if !version.IsDir() {
				continue
			}
			versionPath := fmt.Sprintf("%s/%s", collectionPath, version.Name())

			lang, err := DetectLanguage(versionPath)
			if err != nil {
				continue
			}

			blockFiles, err := DiscoverBlocks(versionPath)
			if err != nil {
				continue
			}

			for _, blockFile := range blockFiles {
				manifest, err := LoadBlockManifest(blockFile)
				if err != nil {
					continue
				}

				hash, err := ComputeContentHash(versionPath)
				if err != nil {
					hash = ""
				}

				entry := BlockRegistryEntry{
					CollectionName:    collection.Name(),
					CollectionVersion: version.Name(),
					BlockName:         manifest.ID,
					BlockID:           manifest.ID,
					Language:          string(lang),
					Entrypoint:        manifest.Entrypoint,
					InstalledPath:     versionPath,
					ContentHash:       hash,
					Kind:              string(manifest.Kind),
					Network:           manifest.Network,
				}

				if err := r.RegisterBlock(entry); err != nil {
					continue
				}
			}
		}
	}

	return nil
}

// VerifyBlock recomputes the content hash and compares it against the stored hash.
func (r *BlockRegistry) VerifyBlock(entry BlockRegistryEntry) error {
	hash, err := ComputeContentHash(entry.InstalledPath)
	if err != nil {
		return fmt.Errorf("computing content hash: %w", err)
	}
	if hash != entry.ContentHash {
		return fmt.Errorf("content hash mismatch for block %s: expected %s, got %s",
			entry.BlockID, entry.ContentHash, hash)
	}
	return nil
}

// Close closes the underlying database connection.
func (r *BlockRegistry) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
