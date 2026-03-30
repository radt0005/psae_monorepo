package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func TestInputRefUnmarshalBareUUID(t *testing.T) {
	yamlData := `- 019cf4bc-1111-7000-0000-000000000000
- 019cf4bc-2222-7000-0000-000000000000`

	var refs []InputRef
	if err := yaml.Unmarshal([]byte(yamlData), &refs); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}

	expected1, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	expected2, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")

	if refs[0].ID != expected1 {
		t.Errorf("expected ID %s, got %s", expected1, refs[0].ID)
	}
	if refs[0].Block != nil {
		t.Error("expected Block to be nil for bare reference")
	}
	if refs[1].ID != expected2 {
		t.Errorf("expected ID %s, got %s", expected2, refs[1].ID)
	}
}

func TestInputRefUnmarshalExplicit(t *testing.T) {
	yamlData := `- block: 019cf4bc-1111-7000-0000-000000000000
  output: clipped_raster`

	var refs []InputRef
	if err := yaml.Unmarshal([]byte(yamlData), &refs); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}

	expectedBlock, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	if refs[0].Block == nil {
		t.Fatal("expected Block to be non-nil for explicit reference")
	}
	if *refs[0].Block != expectedBlock {
		t.Errorf("expected Block %s, got %s", expectedBlock, *refs[0].Block)
	}
	if refs[0].Output != "clipped_raster" {
		t.Errorf("expected Output 'clipped_raster', got %q", refs[0].Output)
	}
}

func TestInputRefMarshalRoundTrip(t *testing.T) {
	id1, _ := uuid.Parse("019cf4bc-1111-7000-0000-000000000000")
	id2, _ := uuid.Parse("019cf4bc-2222-7000-0000-000000000000")

	refs := []InputRef{
		{ID: id1}, // bare
		{Block: &id2, Output: "raster"}, // explicit
	}

	data, err := yaml.Marshal(refs)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var roundTripped []InputRef
	if err := yaml.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("unmarshal round-trip failed: %v", err)
	}

	if len(roundTripped) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(roundTripped))
	}

	// Bare reference
	if roundTripped[0].Block != nil {
		t.Error("expected bare reference Block to be nil after round-trip")
	}
	if roundTripped[0].ID != id1 {
		t.Errorf("expected bare ref ID %s, got %s", id1, roundTripped[0].ID)
	}

	// Explicit reference
	if roundTripped[1].Block == nil {
		t.Fatal("expected explicit reference Block to be non-nil after round-trip")
	}
	if *roundTripped[1].Block != id2 {
		t.Errorf("expected explicit ref Block %s, got %s", id2, *roundTripped[1].Block)
	}
	if roundTripped[1].Output != "raster" {
		t.Errorf("expected Output 'raster', got %q", roundTripped[1].Output)
	}
}

func TestLoadBlockManifest(t *testing.T) {
	dir := t.TempDir()
	manifestYAML := `id: gdal.rasterize
version: 1.0.0
kind: map
network: true
description: Converts vector geometries to raster format
entrypoint: rasterize

inputs:
  vectors:
    type: file
    format: GeoJSON
    description: Vector file
  resolution:
    type: number
    description: Output pixel size

outputs:
  raster:
    type: file
    format: GeoTIFF
    description: Rasterized output
  manifest:
    type: expansion
    description: Expansion manifest
`
	path := filepath.Join(dir, "block.yaml")
	if err := os.WriteFile(path, []byte(manifestYAML), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadBlockManifest(path)
	if err != nil {
		t.Fatalf("LoadBlockManifest failed: %v", err)
	}

	if m.ID != "gdal.rasterize" {
		t.Errorf("expected ID 'gdal.rasterize', got %q", m.ID)
	}
	if m.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got %q", m.Version)
	}
	if m.Kind != BlockKindMap {
		t.Errorf("expected Kind 'map', got %q", m.Kind)
	}
	if !m.Network {
		t.Error("expected Network true")
	}
	if m.Entrypoint != "rasterize" {
		t.Errorf("expected Entrypoint 'rasterize', got %q", m.Entrypoint)
	}
	if len(m.Inputs) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(m.Inputs))
	}
	if m.Inputs["vectors"].Type != "file" {
		t.Errorf("expected vectors type 'file', got %q", m.Inputs["vectors"].Type)
	}
	if m.Inputs["vectors"].Format != "GeoJSON" {
		t.Errorf("expected vectors format 'GeoJSON', got %q", m.Inputs["vectors"].Format)
	}
	if len(m.Outputs) != 2 {
		t.Errorf("expected 2 outputs, got %d", len(m.Outputs))
	}
	if m.Outputs["raster"].Type != "file" {
		t.Errorf("expected raster output type 'file', got %q", m.Outputs["raster"].Type)
	}
}

func TestLoadBlockManifestDefaultKind(t *testing.T) {
	dir := t.TempDir()
	manifestYAML := `id: test.block
version: 1.0.0
inputs:
  data:
    type: file
outputs:
  result:
    type: file
`
	path := filepath.Join(dir, "block.yaml")
	os.WriteFile(path, []byte(manifestYAML), 0644)

	m, err := LoadBlockManifest(path)
	if err != nil {
		t.Fatalf("LoadBlockManifest failed: %v", err)
	}
	if m.Kind != BlockKindStandard {
		t.Errorf("expected default Kind 'standard', got %q", m.Kind)
	}
}

func TestLoadPipeline(t *testing.T) {
	dir := t.TempDir()
	pipelineYAML := `id: 019cf4bc-0000-7000-0000-000000000000
name: test-pipeline
version: "1.0"
description: A test pipeline

blocks:
  - id: 019cf4bc-1111-7000-0000-000000000000
    name: data.sentinel2
    inputs: []
    args:
      region: "POLYGON((...))"

  - id: 019cf4bc-2222-7000-0000-000000000000
    name: raster.reproject
    inputs:
      - 019cf4bc-1111-7000-0000-000000000000
    args:
      target_crs: "EPSG:4326"

  - id: 019cf4bc-3333-7000-0000-000000000000
    name: raster.clip
    inputs:
      - block: 019cf4bc-2222-7000-0000-000000000000
        output: clipped_raster
    args:
      boundary: "area_of_interest.geojson"
`
	path := filepath.Join(dir, "pipeline.yaml")
	os.WriteFile(path, []byte(pipelineYAML), 0644)

	p, err := LoadPipeline(path)
	if err != nil {
		t.Fatalf("LoadPipeline failed: %v", err)
	}

	expectedID, _ := uuid.Parse("019cf4bc-0000-7000-0000-000000000000")
	if p.Id != expectedID {
		t.Errorf("expected pipeline ID %s, got %s", expectedID, p.Id)
	}
	if p.Name != "test-pipeline" {
		t.Errorf("expected name 'test-pipeline', got %q", p.Name)
	}
	if p.Version != "1.0" {
		t.Errorf("expected version '1.0', got %q", p.Version)
	}
	if p.Description != "A test pipeline" {
		t.Errorf("expected description 'A test pipeline', got %q", p.Description)
	}
	if len(p.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(p.Blocks))
	}

	// First block: source, no inputs
	if len(p.Blocks[0].Inputs) != 0 {
		t.Errorf("expected 0 inputs for first block, got %d", len(p.Blocks[0].Inputs))
	}
	if p.Blocks[0].Name != "data.sentinel2" {
		t.Errorf("expected name 'data.sentinel2', got %q", p.Blocks[0].Name)
	}

	// Second block: bare reference
	if len(p.Blocks[1].Inputs) != 1 {
		t.Fatalf("expected 1 input for second block, got %d", len(p.Blocks[1].Inputs))
	}
	if p.Blocks[1].Inputs[0].Block != nil {
		t.Error("expected bare reference for second block input")
	}

	// Third block: explicit reference
	if len(p.Blocks[2].Inputs) != 1 {
		t.Fatalf("expected 1 input for third block, got %d", len(p.Blocks[2].Inputs))
	}
	if p.Blocks[2].Inputs[0].Block == nil {
		t.Fatal("expected explicit reference for third block input")
	}
	if p.Blocks[2].Inputs[0].Output != "clipped_raster" {
		t.Errorf("expected output 'clipped_raster', got %q", p.Blocks[2].Inputs[0].Output)
	}
}

func TestLoadExpansionManifest(t *testing.T) {
	dir := t.TempDir()
	expansionYAML := `items:
  - path: inputs/source/tile_001.tif
    key: tile_001
  - path: inputs/source/tile_002.tif
    key: tile_002
  - path: inputs/source/tile_003.tif
    key: tile_003
`
	path := filepath.Join(dir, "expansion.yaml")
	os.WriteFile(path, []byte(expansionYAML), 0644)

	m, err := LoadExpansionManifest(path)
	if err != nil {
		t.Fatalf("LoadExpansionManifest failed: %v", err)
	}

	if len(m.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(m.Items))
	}
	if m.Items[0].Path != "inputs/source/tile_001.tif" {
		t.Errorf("expected path 'inputs/source/tile_001.tif', got %q", m.Items[0].Path)
	}
	if m.Items[0].Key != "tile_001" {
		t.Errorf("expected key 'tile_001', got %q", m.Items[0].Key)
	}
	if m.Items[2].Key != "tile_003" {
		t.Errorf("expected key 'tile_003', got %q", m.Items[2].Key)
	}
}
