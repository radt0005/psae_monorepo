package spade

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Constructor tests
// ---------------------------------------------------------------------------

func TestFileNew(t *testing.T) {
	f := NewFile("/tmp/data.tif")
	if f.Path != "/tmp/data.tif" {
		t.Fatalf("expected /tmp/data.tif, got %s", f.Path)
	}
}

func TestRasterFileNew(t *testing.T) {
	f := NewRasterFile("/tmp/raster.tif")
	if f.Path != "/tmp/raster.tif" {
		t.Fatalf("expected /tmp/raster.tif, got %s", f.Path)
	}
}

func TestVectorFileNew(t *testing.T) {
	f := NewVectorFile("/tmp/vector.geojson")
	if f.Path != "/tmp/vector.geojson" {
		t.Fatalf("expected /tmp/vector.geojson, got %s", f.Path)
	}
}

func TestTabularFileNew(t *testing.T) {
	f := NewTabularFile("/tmp/data.csv")
	if f.Path != "/tmp/data.csv" {
		t.Fatalf("expected /tmp/data.csv, got %s", f.Path)
	}
}

func TestJsonFileNew(t *testing.T) {
	f := NewJsonFile("/tmp/data.json")
	if f.Path != "/tmp/data.json" {
		t.Fatalf("expected /tmp/data.json, got %s", f.Path)
	}
}

func TestDirectoryNew(t *testing.T) {
	d := NewDirectory("/tmp/source")
	if d.Path != "/tmp/source" {
		t.Fatalf("expected /tmp/source, got %s", d.Path)
	}
}

func TestFileCollectionNew(t *testing.T) {
	fc := NewFileCollection([]string{"/tmp/a.tif", "/tmp/b.tif"})
	if len(fc.Paths) != 2 || fc.Paths[0] != "/tmp/a.tif" || fc.Paths[1] != "/tmp/b.tif" {
		t.Fatalf("unexpected paths: %v", fc.Paths)
	}
}

func TestRasterFileCollectionNew(t *testing.T) {
	rc := NewRasterFileCollection([]string{"/tmp/a.tif", "/tmp/b.tif"})
	if len(rc.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(rc.Paths))
	}
}

func TestVectorFileCollectionNew(t *testing.T) {
	vc := NewVectorFileCollection([]string{"/tmp/a.geojson"})
	if len(vc.Paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(vc.Paths))
	}
}

func TestTabularFileCollectionNew(t *testing.T) {
	tc := NewTabularFileCollection([]string{"/tmp/a.csv", "/tmp/b.csv"})
	if len(tc.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(tc.Paths))
	}
}

func TestEmptyCollection(t *testing.T) {
	fc := NewFileCollection([]string{})
	if len(fc.Paths) != 0 {
		t.Fatalf("expected empty, got %d", len(fc.Paths))
	}
}

// ---------------------------------------------------------------------------
// TypeName / DefaultOutputName / ManifestEntry tests
// ---------------------------------------------------------------------------

func TestTypeNames(t *testing.T) {
	tests := []struct {
		name     string
		st       SpadeType
		typeName string
	}{
		{"File", &File{}, "file"},
		{"RasterFile", &RasterFile{}, "file"},
		{"VectorFile", &VectorFile{}, "file"},
		{"TabularFile", &TabularFile{}, "file"},
		{"JsonFile", &JsonFile{}, "json"},
		{"Directory", &Directory{}, "directory"},
		{"FileCollection", &FileCollection{}, "collection"},
		{"RasterFileCollection", &RasterFileCollection{}, "collection"},
		{"VectorFileCollection", &VectorFileCollection{}, "collection"},
		{"TabularFileCollection", &TabularFileCollection{}, "collection"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.st.TypeName(); got != tt.typeName {
				t.Errorf("TypeName() = %q, want %q", got, tt.typeName)
			}
		})
	}
}

func TestDefaultOutputNames(t *testing.T) {
	tests := []struct {
		name       string
		st         SpadeType
		outputName string
	}{
		{"File", &File{}, "file"},
		{"RasterFile", &RasterFile{}, "raster"},
		{"VectorFile", &VectorFile{}, "vector"},
		{"TabularFile", &TabularFile{}, "tabular"},
		{"JsonFile", &JsonFile{}, "json"},
		{"Directory", &Directory{}, "directory"},
		{"FileCollection", &FileCollection{}, "files"},
		{"RasterFileCollection", &RasterFileCollection{}, "rasters"},
		{"VectorFileCollection", &VectorFileCollection{}, "vectors"},
		{"TabularFileCollection", &TabularFileCollection{}, "tables"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.st.DefaultOutputName(); got != tt.outputName {
				t.Errorf("DefaultOutputName() = %q, want %q", got, tt.outputName)
			}
		})
	}
}

func TestManifestEntries(t *testing.T) {
	tests := []struct {
		name string
		st   SpadeType
		want ManifestInfo
	}{
		{"File", &File{}, ManifestInfo{TypeName: "file"}},
		{"RasterFile", &RasterFile{}, ManifestInfo{TypeName: "file", Format: "GeoTIFF"}},
		{"VectorFile", &VectorFile{}, ManifestInfo{TypeName: "file", Format: "GeoJSON"}},
		{"TabularFile", &TabularFile{}, ManifestInfo{TypeName: "file", Format: "CSV"}},
		{"JsonFile", &JsonFile{}, ManifestInfo{TypeName: "json"}},
		{"Directory", &Directory{}, ManifestInfo{TypeName: "directory"}},
		{"FileCollection", &FileCollection{}, ManifestInfo{TypeName: "collection", ItemType: "file"}},
		{"RasterFileCollection", &RasterFileCollection{}, ManifestInfo{TypeName: "collection", Format: "GeoTIFF", ItemType: "file"}},
		{"VectorFileCollection", &VectorFileCollection{}, ManifestInfo{TypeName: "collection", Format: "GeoJSON", ItemType: "file"}},
		{"TabularFileCollection", &TabularFileCollection{}, ManifestInfo{TypeName: "collection", Format: "CSV", ItemType: "file"}},
		{"StringType", StringType{}, ManifestInfo{TypeName: "string"}},
		{"NumberType", NumberType{}, ManifestInfo{TypeName: "number"}},
		{"BoolType", BoolType{}, ManifestInfo{TypeName: "boolean"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.st.ManifestEntry()
			if got != tt.want {
				t.Errorf("ManifestEntry() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FromInput tests
// ---------------------------------------------------------------------------

func TestFileFromSingleFile(t *testing.T) {
	var f File
	if err := f.FromSingleFile("/tmp/data.tif"); err != nil {
		t.Fatal(err)
	}
	if f.Path != "/tmp/data.tif" {
		t.Fatalf("expected /tmp/data.tif, got %s", f.Path)
	}
}

func TestFileFromMultipleFiles(t *testing.T) {
	var f File
	if err := f.FromMultipleFiles([]string{"/tmp/a.tif", "/tmp/b.tif"}); err != nil {
		t.Fatal(err)
	}
	if f.Path != "/tmp/a.tif" {
		t.Fatalf("expected /tmp/a.tif, got %s", f.Path)
	}
}

func TestFileFromDirectoryError(t *testing.T) {
	var f File
	err := f.FromDirectory("/tmp/dir")
	if err == nil {
		t.Fatal("expected error")
	}
	var tmErr *ErrTypeMismatch
	if !errors.As(err, &tmErr) {
		t.Fatalf("expected ErrTypeMismatch, got %T", err)
	}
}

func TestDirectoryFromSingleFileError(t *testing.T) {
	var d Directory
	err := d.FromSingleFile("/tmp/file.tif")
	if err == nil {
		t.Fatal("expected error")
	}
	var tmErr *ErrTypeMismatch
	if !errors.As(err, &tmErr) {
		t.Fatalf("expected ErrTypeMismatch, got %T", err)
	}
}

func TestDirectoryFromMultipleFilesError(t *testing.T) {
	var d Directory
	err := d.FromMultipleFiles([]string{"/tmp/a", "/tmp/b"})
	if err == nil {
		t.Fatal("expected error")
	}
	var tmErr *ErrTypeMismatch
	if !errors.As(err, &tmErr) {
		t.Fatalf("expected ErrTypeMismatch, got %T", err)
	}
}

func TestDirectoryFromDirectory(t *testing.T) {
	var d Directory
	if err := d.FromDirectory("/tmp/source"); err != nil {
		t.Fatal(err)
	}
	if d.Path != "/tmp/source" {
		t.Fatalf("expected /tmp/source, got %s", d.Path)
	}
}

func TestCollectionFromSingleFile(t *testing.T) {
	var fc FileCollection
	if err := fc.FromSingleFile("/tmp/a.tif"); err != nil {
		t.Fatal(err)
	}
	if len(fc.Paths) != 1 || fc.Paths[0] != "/tmp/a.tif" {
		t.Fatalf("unexpected paths: %v", fc.Paths)
	}
}

func TestCollectionFromMultipleFiles(t *testing.T) {
	var fc FileCollection
	if err := fc.FromMultipleFiles([]string{"/tmp/a.tif", "/tmp/b.tif"}); err != nil {
		t.Fatal(err)
	}
	if len(fc.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(fc.Paths))
	}
}

func TestCollectionFromDirectoryError(t *testing.T) {
	var fc FileCollection
	err := fc.FromDirectory("/tmp/dir")
	if err == nil {
		t.Fatal("expected error")
	}
	var tmErr *ErrTypeMismatch
	if !errors.As(err, &tmErr) {
		t.Fatalf("expected ErrTypeMismatch, got %T", err)
	}
}

// ---------------------------------------------------------------------------
// WriteTo tests
// ---------------------------------------------------------------------------

func TestFileWriteTo(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data.tif")
	os.WriteFile(src, []byte("test data"), 0644)

	outputDir := filepath.Join(dir, "output")
	f := NewFile(src)
	if err := f.WriteTo(outputDir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "data.tif"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "test data" {
		t.Fatalf("expected 'test data', got %q", string(data))
	}
}

func TestRasterFileWriteTo(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "raster.tif")
	os.WriteFile(src, []byte("raster data"), 0644)

	outputDir := filepath.Join(dir, "output")
	f := NewRasterFile(src)
	if err := f.WriteTo(outputDir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "raster.tif"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "raster data" {
		t.Fatalf("expected 'raster data', got %q", string(data))
	}
}

func TestDirectoryWriteTo(t *testing.T) {
	dir := t.TempDir()

	// Create source directory with files and subdirectory
	srcDir := filepath.Join(dir, "source")
	os.MkdirAll(filepath.Join(srcDir, "sub"), 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(srcDir, "sub", "file3.txt"), []byte("c"), 0644)

	outputDir := filepath.Join(dir, "output")
	d := NewDirectory(srcDir)
	if err := d.WriteTo(outputDir); err != nil {
		t.Fatal(err)
	}

	// Verify files
	for _, tc := range []struct {
		path    string
		content string
	}{
		{"file1.txt", "a"},
		{"file2.txt", "b"},
		{filepath.Join("sub", "file3.txt"), "c"},
	} {
		data, err := os.ReadFile(filepath.Join(outputDir, tc.path))
		if err != nil {
			t.Fatalf("reading %s: %v", tc.path, err)
		}
		if string(data) != tc.content {
			t.Fatalf("%s: expected %q, got %q", tc.path, tc.content, string(data))
		}
	}
}

func TestFileCollectionWriteTo(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for i, name := range []string{"a.tif", "b.tif", "c.tif"} {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte{byte(i)}, 0644)
		paths = append(paths, p)
	}

	outputDir := filepath.Join(dir, "output")
	fc := NewFileCollection(paths)
	if err := fc.WriteTo(outputDir); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(outputDir)
	if len(entries) != 3 {
		t.Fatalf("expected 3 files, got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// Value equality tests
// ---------------------------------------------------------------------------

func TestValueEquality(t *testing.T) {
	f1 := NewFile("/tmp/data.tif")
	f2 := NewFile("/tmp/data.tif")
	if f1 != f2 {
		t.Fatal("expected equal File values")
	}

	d1 := NewDirectory("/tmp/source")
	d2 := NewDirectory("/tmp/source")
	if d1 != d2 {
		t.Fatal("expected equal Directory values")
	}
}
