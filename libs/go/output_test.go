package spade

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// WriteOutputs tests
// ---------------------------------------------------------------------------

func TestWriteSingleRasterFile(t *testing.T) {
	dir := setupWorkDir(t)
	src := filepath.Join(dir, "tmp_result.tif")
	os.WriteFile(src, []byte("raster data"), 0644)

	result := NewRasterFile(src)
	if err := WriteOutputs(&result, dir, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "outputs", "raster", "tmp_result.tif"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "raster data" {
		t.Fatalf("expected 'raster data', got %q", string(data))
	}
}

func TestWriteSingleFile(t *testing.T) {
	dir := setupWorkDir(t)
	src := filepath.Join(dir, "result.dat")
	os.WriteFile(src, []byte("file data"), 0644)

	result := NewFile(src)
	if err := WriteOutputs(&result, dir, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "outputs", "file", "result.dat")); err != nil {
		t.Fatal("expected output file to exist")
	}
}

func TestWriteRasterFileCollection(t *testing.T) {
	dir := setupWorkDir(t)
	var paths []string
	for i, name := range []string{"tile_0.tif", "tile_1.tif", "tile_2.tif"} {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte{byte(i)}, 0644)
		paths = append(paths, p)
	}

	result := NewRasterFileCollection(paths)
	if err := WriteOutputs(&result, dir, nil); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(filepath.Join(dir, "outputs", "rasters"))
	if len(entries) != 3 {
		t.Fatalf("expected 3 files, got %d", len(entries))
	}
}

func TestWriteDirectoryOutput(t *testing.T) {
	dir := setupWorkDir(t)
	srcDir := filepath.Join(dir, "result_dir")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("b"), 0644)

	result := NewDirectory(srcDir)
	if err := WriteOutputs(&result, dir, nil); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(dir, "outputs", "directory")
	if _, err := os.Stat(filepath.Join(outputDir, "file1.txt")); err != nil {
		t.Fatal("file1.txt not found")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "file2.txt")); err != nil {
		t.Fatal("file2.txt not found")
	}
}

func TestWriteWithManifestCustomName(t *testing.T) {
	dir := setupWorkDir(t)
	src := filepath.Join(dir, "result.tif")
	os.WriteFile(src, []byte("data"), 0644)

	manifest := map[string]any{
		"custom_output": map[string]any{"type": "file"},
	}

	result := NewRasterFile(src)
	if err := WriteOutputs(&result, dir, manifest); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "outputs", "custom_output", "result.tif")); err != nil {
		t.Fatal("expected file at custom_output/result.tif")
	}
}

func TestWriteMultipleNamedOutputs(t *testing.T) {
	dir := setupWorkDir(t)
	rasterSrc := filepath.Join(dir, "result.tif")
	os.WriteFile(rasterSrc, []byte("raster"), 0644)
	jsonSrc := filepath.Join(dir, "summary.json")
	os.WriteFile(jsonSrc, []byte(`{"key": "value"}`), 0644)

	outputs := NewOutputs()
	r := NewRasterFile(rasterSrc)
	j := NewJsonFile(jsonSrc)
	outputs.Add("raster", &r)
	outputs.Add("summary", &j)

	if err := WriteOutputs(outputs, dir, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "outputs", "raster", "result.tif")); err != nil {
		t.Fatal("raster output not found")
	}
	if _, err := os.Stat(filepath.Join(dir, "outputs", "summary", "summary.json")); err != nil {
		t.Fatal("summary output not found")
	}
}

func TestWriteUnitOutput(t *testing.T) {
	dir := setupWorkDir(t)
	if err := WriteOutputs(unitOutput{}, dir, nil); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "outputs"))
	if len(entries) != 0 {
		t.Fatalf("expected empty outputs, got %d entries", len(entries))
	}
}

func TestFilenamePreservation(t *testing.T) {
	dir := setupWorkDir(t)
	src := filepath.Join(dir, "my_custom_name.geojson")
	os.WriteFile(src, []byte("geojson data"), 0644)

	result := NewVectorFile(src)
	if err := WriteOutputs(&result, dir, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "outputs", "vector", "my_custom_name.geojson")); err != nil {
		t.Fatal("expected original filename preserved")
	}
}

// ---------------------------------------------------------------------------
// ReadBlockManifest tests
// ---------------------------------------------------------------------------

func TestReadManifestNone(t *testing.T) {
	dir := setupWorkDir(t)
	t.Setenv("SPADE_BLOCK_MANIFEST", "")
	result := ReadBlockManifest(dir)
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestReadManifestBlockYaml(t *testing.T) {
	dir := setupWorkDir(t)
	t.Setenv("SPADE_BLOCK_MANIFEST", "")
	content := "id: test.block\noutputs:\n  raster:\n    type: file\n    format: GeoTIFF\n"
	os.WriteFile(filepath.Join(dir, "block.yaml"), []byte(content), 0644)

	result := ReadBlockManifest(dir)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if _, ok := result["raster"]; !ok {
		t.Fatal("expected 'raster' key")
	}
}

func TestReadManifestEnvVar(t *testing.T) {
	dir := setupWorkDir(t)
	manifestPath := filepath.Join(dir, "external_block.yaml")
	content := "outputs:\n  output:\n    type: file\n"
	os.WriteFile(manifestPath, []byte(content), 0644)

	t.Setenv("SPADE_BLOCK_MANIFEST", manifestPath)
	result := ReadBlockManifest(dir)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if _, ok := result["output"]; !ok {
		t.Fatal("expected 'output' key")
	}
}

func TestReadManifestEnvVarTakesPrecedence(t *testing.T) {
	dir := setupWorkDir(t)

	// block.yaml in work dir
	cwdContent := "outputs:\n  cwd_output:\n    type: file\n"
	os.WriteFile(filepath.Join(dir, "block.yaml"), []byte(cwdContent), 0644)

	// external manifest
	extPath := filepath.Join(dir, "external.yaml")
	extContent := "outputs:\n  env_output:\n    type: file\n"
	os.WriteFile(extPath, []byte(extContent), 0644)

	t.Setenv("SPADE_BLOCK_MANIFEST", extPath)
	result := ReadBlockManifest(dir)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if _, ok := result["env_output"]; !ok {
		t.Fatal("expected 'env_output' key")
	}
	if _, ok := result["cwd_output"]; ok {
		t.Fatal("should not contain 'cwd_output' — env var takes precedence")
	}
}
