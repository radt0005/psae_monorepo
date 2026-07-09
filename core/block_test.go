package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func TestCreateBlockDirectory(t *testing.T) {
	dir := t.TempDir()
	id := uuid.New()

	if err := CreateBlockDirectory(id.String(), dir); err != nil {
		t.Fatalf("CreateBlockDirectory failed: %v", err)
	}

	base := filepath.Join(dir, id.String())
	for _, sub := range []string{"", "inputs", "outputs", "logs"} {
		p := filepath.Join(base, sub)
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", p, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", p)
		}
	}
}

func TestWriteParamsYAML(t *testing.T) {
	dir := t.TempDir()
	args := map[string]any{
		"buffer_distance": 30,
		"method":          "bilinear",
		"normalize":       true,
		"nested": map[string]any{
			"key": "value",
		},
	}

	if err := WriteParamsYAML(args, dir); err != nil {
		t.Fatalf("WriteParamsYAML failed: %v", err)
	}

	path := filepath.Join(dir, "params.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading params.yaml: %v", err)
	}

	// Verify we can parse it back
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parsing params.yaml: %v", err)
	}

	if parsed["method"] != "bilinear" {
		t.Errorf("expected method 'bilinear', got %v", parsed["method"])
	}
	if parsed["normalize"] != true {
		t.Errorf("expected normalize true, got %v", parsed["normalize"])
	}
}

func TestSetupInputSymlinks(t *testing.T) {
	dir := t.TempDir()
	pipelineDir := filepath.Join(dir, "pipeline")
	workDir := filepath.Join(dir, "work")
	os.MkdirAll(filepath.Join(workDir, "inputs"), 0755)

	// Create a mock dependency output
	depID := uuid.New()
	depOutputDir := filepath.Join(pipelineDir, depID.String(), "outputs", "raster")
	os.MkdirAll(depOutputDir, 0755)
	os.WriteFile(filepath.Join(depOutputDir, "data.tif"), []byte("mock tif"), 0644)

	resolved := map[string]ResolvedInput{
		"source": {
			InputName:        "source",
			SourceBlockID:    depID,
			SourceOutputName: "raster",
		},
	}

	if err := SetupInputSymlinks(
		workDir, resolved, pipelineDir,
		BlockInvocation{}, BlockManifest{}, map[uuid.UUID]BlockManifest{}, nil,
	); err != nil {
		t.Fatalf("SetupInputSymlinks failed: %v", err)
	}

	// Verify a symlink exists for each file that lived in the source
	// output directory.
	linkPath := filepath.Join(workDir, "inputs", "source", "data.tif")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("expected symlink at %s: %v", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected a symlink")
	}
}

func TestCollectOutputs(t *testing.T) {
	dir := t.TempDir()
	outputsDir := filepath.Join(dir, "outputs")
	os.MkdirAll(filepath.Join(outputsDir, "raster"), 0755)
	os.WriteFile(filepath.Join(outputsDir, "raster", "data.tif"), []byte("mock tif data"), 0644)
	os.MkdirAll(filepath.Join(outputsDir, "summary"), 0755)
	os.WriteFile(filepath.Join(outputsDir, "summary", "summary.json"), []byte(`{"count": 42}`), 0644)

	hashes, err := CollectOutputs(dir)
	if err != nil {
		t.Fatalf("CollectOutputs failed: %v", err)
	}

	if len(hashes) != 2 {
		t.Fatalf("expected 2 outputs, got %d", len(hashes))
	}
	if _, ok := hashes["raster"]; !ok {
		t.Error("expected 'raster' in output hashes")
	}
	if _, ok := hashes["summary"]; !ok {
		t.Error("expected 'summary' in output hashes")
	}
	// Hashes should be non-empty hex strings
	for name, hash := range hashes {
		if len(hash) != 64 { // SHA-256 hex length
			t.Errorf("expected 64-char hash for %s, got %d chars: %s", name, len(hash), hash)
		}
	}
}

func TestWriteInvocationMetadata(t *testing.T) {
	dir := t.TempDir()
	meta := InvocationMetadata{
		Block: InvocationMetadataBlock{
			ID:      "raster.reproject",
			Version: "1.0.0",
		},
		InvocationID: "019cf4bc-1111-7000-0000-000000000000",
		Inputs: map[string]InvocationMetadataInput{
			"reference": {Path: "inputs/reference/data.tif", Hash: "abc123"},
			"target":    {Path: "inputs/target/data.tif", Hash: "def456"},
		},
	}

	if err := WriteInvocationMetadata(meta, dir); err != nil {
		t.Fatalf("WriteInvocationMetadata failed: %v", err)
	}

	path := filepath.Join(dir, "invocation.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading invocation.yaml: %v", err)
	}

	var parsed InvocationMetadata
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parsing invocation.yaml: %v", err)
	}

	if parsed.Block.ID != "raster.reproject" {
		t.Errorf("expected block ID 'raster.reproject', got %q", parsed.Block.ID)
	}
	if parsed.Block.Version != "1.0.0" {
		t.Errorf("expected block version '1.0.0', got %q", parsed.Block.Version)
	}
	if parsed.InvocationID != "019cf4bc-1111-7000-0000-000000000000" {
		t.Errorf("expected invocation ID, got %q", parsed.InvocationID)
	}
	if len(parsed.Inputs) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(parsed.Inputs))
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		file     string
		expected CollectionLanguage
	}{
		{"Cargo.toml", CollectionLanguageRust},
		{"go.mod", CollectionLanguageGo},
		{"pyproject.toml", CollectionLanguagePython},
		{"package.json", CollectionLanguageTypeScript},
	}

	for _, tc := range tests {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, tc.file), []byte(""), 0644)

		lang, err := DetectLanguage(dir)
		if err != nil {
			t.Errorf("DetectLanguage failed for %s: %v", tc.file, err)
			continue
		}
		if lang != tc.expected {
			t.Errorf("for %s: expected %s, got %s", tc.file, tc.expected, lang)
		}
	}

	// Test R default
	dir := t.TempDir()
	lang, err := DetectLanguage(dir)
	if err != nil {
		t.Fatalf("DetectLanguage for R default failed: %v", err)
	}
	if lang != CollectionLanguageR {
		t.Errorf("expected R default, got %s", lang)
	}
}

func TestDiscoverBlocks(t *testing.T) {
	dir := t.TempDir()
	blocksDir := filepath.Join(dir, "blocks")
	os.MkdirAll(blocksDir, 0755)

	os.WriteFile(filepath.Join(blocksDir, "rasterize.yaml"), []byte("id: test.rasterize"), 0644)
	os.WriteFile(filepath.Join(blocksDir, "reproject.yaml"), []byte("id: test.reproject"), 0644)
	os.WriteFile(filepath.Join(blocksDir, "README.md"), []byte("not a block"), 0644)

	paths, err := DiscoverBlocks(dir)
	if err != nil {
		t.Fatalf("DiscoverBlocks failed: %v", err)
	}

	if len(paths) != 2 {
		t.Fatalf("expected 2 block paths, got %d", len(paths))
	}
}

func TestComputeContentHash(t *testing.T) {
	dir := t.TempDir()
	content := "test content for hashing"
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte(content), 0644)

	hash1, err := ComputeContentHash(path)
	if err != nil {
		t.Fatalf("ComputeContentHash failed: %v", err)
	}
	if len(hash1) != 64 {
		t.Errorf("expected 64-char hash, got %d chars", len(hash1))
	}

	// Same content should produce same hash
	path2 := filepath.Join(dir, "test2.txt")
	os.WriteFile(path2, []byte(content), 0644)
	hash2, _ := ComputeContentHash(path2)
	if hash1 != hash2 {
		t.Error("same content should produce same hash")
	}

	// Different content should produce different hash
	path3 := filepath.Join(dir, "test3.txt")
	os.WriteFile(path3, []byte("different"), 0644)
	hash3, _ := ComputeContentHash(path3)
	if hash1 == hash3 {
		t.Error("different content should produce different hash")
	}

	// Directory hashing
	subdir := filepath.Join(dir, "subdir")
	os.MkdirAll(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(subdir, "b.txt"), []byte("bbb"), 0644)
	dirHash, err := ComputeContentHash(subdir)
	if err != nil {
		t.Fatalf("ComputeContentHash for dir failed: %v", err)
	}
	if len(dirHash) != 64 {
		t.Errorf("expected 64-char hash for dir, got %d chars", len(dirHash))
	}
}

func TestResolveEntrypoint(t *testing.T) {
	tests := []struct {
		name     string
		entry    BlockRegistryEntry
		wantExec string
		wantArgs []string
	}{
		{
			name: "go uses collection binary with block subcommand",
			entry: BlockRegistryEntry{
				Language:       "go",
				CollectionName: "gdal",
				BlockName:      "rasterize",
				InstalledPath:  "/blocks/gdal",
			},
			wantExec: "/blocks/gdal/gdal",
			wantArgs: []string{"rasterize"},
		},
		{
			name: "rust uses collection binary with block subcommand",
			entry: BlockRegistryEntry{
				Language:       "rust",
				CollectionName: "gdal",
				BlockName:      "rasterize",
				InstalledPath:  "/blocks/gdal",
			},
			wantExec: "/blocks/gdal/gdal",
			wantArgs: []string{"rasterize"},
		},
		{
			name: "typescript uses bundled binary with block subcommand",
			entry: BlockRegistryEntry{
				Language:       "typescript",
				CollectionName: "gdal",
				BlockName:      "rasterize",
				InstalledPath:  "/blocks/gdal",
			},
			wantExec: "/blocks/gdal/gdal",
			wantArgs: []string{"rasterize"},
		},
		{
			name: "python defaults module to collection package",
			entry: BlockRegistryEntry{
				Language:       "python",
				CollectionName: "my-coll",
				BlockName:      "read",
				InstalledPath:  "/blocks/my-coll",
			},
			wantExec: "uv",
			wantArgs: []string{"run", "--project", "/blocks/my-coll", "--no-sync", "-m", "my_coll.read"},
		},
		{
			name: "python passes qualified module through verbatim",
			entry: BlockRegistryEntry{
				Language:       "python",
				CollectionName: "my-coll",
				BlockName:      "read",
				Entrypoint:     "pkg.sub.read",
				InstalledPath:  "/blocks/my-coll",
			},
			wantExec: "uv",
			wantArgs: []string{"run", "--project", "/blocks/my-coll", "--no-sync", "-m", "pkg.sub.read"},
		},
		{
			name: "r expands block-name default to absolute script path",
			entry: BlockRegistryEntry{
				Language:      "r",
				BlockName:     "hello_world",
				InstalledPath: "/blocks/phils-code",
			},
			wantExec: "Rscript",
			wantArgs: []string{"/blocks/phils-code/R/hello_world.R"},
		},
		{
			name: "r honors explicit entrypoint",
			entry: BlockRegistryEntry{
				Language:      "r",
				BlockName:     "hello_world",
				Entrypoint:    "greet",
				InstalledPath: "/blocks/phils-code",
			},
			wantExec: "Rscript",
			wantArgs: []string{"/blocks/phils-code/R/greet.R"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, args, err := ResolveEntrypoint(tt.entry)
			if err != nil {
				t.Fatalf("ResolveEntrypoint returned error: %v", err)
			}
			if exec != tt.wantExec {
				t.Errorf("exec = %q, want %q", exec, tt.wantExec)
			}
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("args = %v, want %v", args, tt.wantArgs)
			}
			for i := range args {
				if args[i] != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, args[i], tt.wantArgs[i])
				}
			}
		})
	}

	if _, _, err := ResolveEntrypoint(BlockRegistryEntry{Language: "cobol"}); err == nil {
		t.Error("expected error for unsupported language, got nil")
	}
}
