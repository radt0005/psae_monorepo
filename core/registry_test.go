package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRegistry(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	registry, err := OpenRegistry(dbPath)
	if err != nil {
		t.Fatalf("OpenRegistry failed: %v", err)
	}
	defer registry.Close()

	// Verify file exists with correct permissions
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("database file not created: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected permissions 0600, got %o", perm)
	}

	// Verify WAL mode
	sqlDB, _ := registry.db.DB()
	var journalMode string
	sqlDB.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if journalMode != "wal" {
		t.Errorf("expected WAL journal mode, got %q", journalMode)
	}
}

func TestRegisterAndLookupBlock(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	registry, err := OpenRegistry(dbPath)
	if err != nil {
		t.Fatalf("OpenRegistry failed: %v", err)
	}
	defer registry.Close()

	entry := BlockRegistryEntry{
		CollectionName:    "gdal-tools",
		CollectionVersion: "1.0.0",
		BlockName:         "rasterize",
		BlockID:           "gdal.rasterize",
		Language:          "rust",
		Entrypoint:        "rasterize",
		InstalledPath:     "/home/user/.spade/blocks/gdal-tools/1.0.0",
		ContentHash:       "abc123def456",
		Kind:              "standard",
		Network:           false,
	}

	if err := registry.RegisterBlock(entry); err != nil {
		t.Fatalf("RegisterBlock failed: %v", err)
	}

	result, err := registry.LookupBlock("gdal.rasterize", "1.0.0")
	if err != nil {
		t.Fatalf("LookupBlock failed: %v", err)
	}

	if result.BlockID != "gdal.rasterize" {
		t.Errorf("expected BlockID 'gdal.rasterize', got %q", result.BlockID)
	}
	if result.Language != "rust" {
		t.Errorf("expected Language 'rust', got %q", result.Language)
	}
	if result.ContentHash != "abc123def456" {
		t.Errorf("expected ContentHash 'abc123def456', got %q", result.ContentHash)
	}
}

func TestLookupBlockVersions(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	registry, err := OpenRegistry(dbPath)
	if err != nil {
		t.Fatalf("OpenRegistry failed: %v", err)
	}
	defer registry.Close()

	// Register two versions
	entry1 := BlockRegistryEntry{
		CollectionName:    "gdal-tools",
		CollectionVersion: "1.0.0",
		BlockName:         "rasterize",
		BlockID:           "gdal.rasterize",
		Language:          "rust",
		InstalledPath:     "/v1",
		ContentHash:       "hash1",
	}
	entry2 := BlockRegistryEntry{
		CollectionName:    "gdal-tools",
		CollectionVersion: "2.0.0",
		BlockName:         "rasterize",
		BlockID:           "gdal.rasterize",
		Language:          "rust",
		InstalledPath:     "/v2",
		ContentHash:       "hash2",
	}

	registry.RegisterBlock(entry1)
	registry.RegisterBlock(entry2)

	// Lookup specific version
	r1, err := registry.LookupBlock("gdal.rasterize", "1.0.0")
	if err != nil {
		t.Fatalf("LookupBlock v1 failed: %v", err)
	}
	if r1.InstalledPath != "/v1" {
		t.Errorf("expected path /v1, got %q", r1.InstalledPath)
	}

	r2, err := registry.LookupBlock("gdal.rasterize", "2.0.0")
	if err != nil {
		t.Fatalf("LookupBlock v2 failed: %v", err)
	}
	if r2.InstalledPath != "/v2" {
		t.Errorf("expected path /v2, got %q", r2.InstalledPath)
	}

	// Lookup without version returns latest (sorted DESC)
	latest, err := registry.LookupBlock("gdal.rasterize", "")
	if err != nil {
		t.Fatalf("LookupBlock latest failed: %v", err)
	}
	if latest.CollectionVersion != "2.0.0" {
		t.Errorf("expected latest version 2.0.0, got %q", latest.CollectionVersion)
	}
}

func TestVerifyBlock(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	registry, err := OpenRegistry(dbPath)
	if err != nil {
		t.Fatalf("OpenRegistry failed: %v", err)
	}
	defer registry.Close()

	// Create a file to hash
	blockDir := filepath.Join(dir, "block")
	os.MkdirAll(blockDir, 0755)
	os.WriteFile(filepath.Join(blockDir, "main.rs"), []byte("fn main() {}"), 0644)

	correctHash, _ := ComputeContentHash(blockDir)

	// Verify with correct hash
	entry := BlockRegistryEntry{
		InstalledPath: blockDir,
		ContentHash:   correctHash,
		BlockID:       "test.block",
	}
	if err := registry.VerifyBlock(entry); err != nil {
		t.Errorf("VerifyBlock should pass with correct hash: %v", err)
	}

	// Verify with wrong hash
	entry.ContentHash = "wrong_hash"
	if err := registry.VerifyBlock(entry); err == nil {
		t.Error("VerifyBlock should fail with wrong hash")
	}
}

func TestListBlocks(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	registry, err := OpenRegistry(dbPath)
	if err != nil {
		t.Fatalf("OpenRegistry failed: %v", err)
	}
	defer registry.Close()

	registry.RegisterBlock(BlockRegistryEntry{BlockID: "a.block", CollectionVersion: "1.0"})
	registry.RegisterBlock(BlockRegistryEntry{BlockID: "b.block", CollectionVersion: "1.0"})

	blocks, err := registry.ListBlocks()
	if err != nil {
		t.Fatalf("ListBlocks failed: %v", err)
	}
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(blocks))
	}
}

func TestDeleteCollection(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	registry, err := OpenRegistry(dbPath)
	if err != nil {
		t.Fatalf("OpenRegistry failed: %v", err)
	}
	defer registry.Close()

	registry.RegisterBlock(BlockRegistryEntry{
		BlockID: "gdal.rasterize", CollectionName: "gdal", CollectionVersion: "1.0",
	})
	registry.RegisterBlock(BlockRegistryEntry{
		BlockID: "gdal.clip", CollectionName: "gdal", CollectionVersion: "1.0",
	})

	if err := registry.DeleteCollection("gdal", "1.0"); err != nil {
		t.Fatalf("DeleteCollection failed: %v", err)
	}

	blocks, _ := registry.ListBlocks()
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks after delete, got %d", len(blocks))
	}
}
