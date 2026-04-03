package spade

import (
	"testing"
)

func getField(manifest map[string]any, section, name, field string) (string, bool) {
	sec, ok := manifest[section].(map[string]any)
	if !ok {
		return "", false
	}
	entry, ok := sec[name].(map[string]any)
	if !ok {
		return "", false
	}
	val, ok := entry[field].(string)
	return val, ok
}

func TestBuildSimpleFileInput(t *testing.T) {
	b := NewManifestBuilder()
	ManifestInput[*File](b, "source")
	m := b.Build()

	typ, ok := getField(m, "inputs", "source", "type")
	if !ok || typ != "file" {
		t.Fatalf("expected type=file, got %q", typ)
	}
}

func TestBuildTypedFileInputs(t *testing.T) {
	b := NewManifestBuilder()
	ManifestInput[*RasterFile](b, "raster")
	ManifestInput[*VectorFile](b, "vector")
	m := b.Build()

	typ, _ := getField(m, "inputs", "raster", "type")
	fmt_, _ := getField(m, "inputs", "raster", "format")
	if typ != "file" || fmt_ != "GeoTIFF" {
		t.Fatalf("raster: expected file/GeoTIFF, got %s/%s", typ, fmt_)
	}

	typ, _ = getField(m, "inputs", "vector", "type")
	fmt_, _ = getField(m, "inputs", "vector", "format")
	if typ != "file" || fmt_ != "GeoJSON" {
		t.Fatalf("vector: expected file/GeoJSON, got %s/%s", typ, fmt_)
	}
}

func TestBuildScalarInputs(t *testing.T) {
	b := NewManifestBuilder()
	ManifestInput[StringType](b, "name")
	ManifestInput[NumberType](b, "count")
	ManifestInput[BoolType](b, "enabled")
	m := b.Build()

	typ, _ := getField(m, "inputs", "name", "type")
	if typ != "string" {
		t.Fatalf("expected string, got %s", typ)
	}
	typ, _ = getField(m, "inputs", "count", "type")
	if typ != "number" {
		t.Fatalf("expected number, got %s", typ)
	}
	typ, _ = getField(m, "inputs", "enabled", "type")
	if typ != "boolean" {
		t.Fatalf("expected boolean, got %s", typ)
	}
}

func TestBuildCollectionInput(t *testing.T) {
	b := NewManifestBuilder()
	ManifestInput[*RasterFileCollection](b, "tiles")
	m := b.Build()

	typ, _ := getField(m, "inputs", "tiles", "type")
	itemType, _ := getField(m, "inputs", "tiles", "item_type")
	fmt_, _ := getField(m, "inputs", "tiles", "format")
	if typ != "collection" {
		t.Fatalf("expected collection, got %s", typ)
	}
	if itemType != "file" {
		t.Fatalf("expected item_type=file, got %s", itemType)
	}
	if fmt_ != "GeoTIFF" {
		t.Fatalf("expected format=GeoTIFF, got %s", fmt_)
	}
}

func TestBuildOutputDeclarations(t *testing.T) {
	b := NewManifestBuilder()
	ManifestInput[*File](b, "source")
	ManifestOutput[*RasterFile](b, "raster")
	m := b.Build()

	typ, _ := getField(m, "outputs", "raster", "type")
	fmt_, _ := getField(m, "outputs", "raster", "format")
	if typ != "file" || fmt_ != "GeoTIFF" {
		t.Fatalf("expected file/GeoTIFF, got %s/%s", typ, fmt_)
	}
}

func TestBuildDescription(t *testing.T) {
	b := NewManifestBuilder()
	b.Description("Processes input data.")
	ManifestInput[*File](b, "source")
	m := b.Build()

	desc, ok := m["description"].(string)
	if !ok || desc != "Processes input data." {
		t.Fatalf("expected 'Processes input data.', got %q", desc)
	}
}

func TestBuildNoDescription(t *testing.T) {
	b := NewManifestBuilder()
	ManifestInput[*File](b, "source")
	m := b.Build()

	if _, ok := m["description"]; ok {
		t.Fatal("expected no description key")
	}
}

func TestBuildMixedInputs(t *testing.T) {
	b := NewManifestBuilder()
	b.Description("Normalizes raster data.")
	ManifestInput[*RasterFile](b, "raster")
	ManifestInput[NumberType](b, "buffer")
	ManifestInput[BoolType](b, "normalize")
	ManifestOutput[*RasterFile](b, "raster")
	m := b.Build()

	typ, _ := getField(m, "inputs", "raster", "type")
	fmt_, _ := getField(m, "inputs", "raster", "format")
	if typ != "file" || fmt_ != "GeoTIFF" {
		t.Fatalf("raster input: expected file/GeoTIFF, got %s/%s", typ, fmt_)
	}

	typ, _ = getField(m, "inputs", "buffer", "type")
	if typ != "number" {
		t.Fatalf("buffer: expected number, got %s", typ)
	}

	typ, _ = getField(m, "inputs", "normalize", "type")
	if typ != "boolean" {
		t.Fatalf("normalize: expected boolean, got %s", typ)
	}

	typ, _ = getField(m, "outputs", "raster", "type")
	if typ != "file" {
		t.Fatalf("raster output: expected file, got %s", typ)
	}

	desc := m["description"].(string)
	if desc != "Normalizes raster data." {
		t.Fatalf("expected 'Normalizes raster data.', got %q", desc)
	}
}

func TestBuildDirectoryInput(t *testing.T) {
	b := NewManifestBuilder()
	ManifestInput[*Directory](b, "source")
	m := b.Build()

	typ, _ := getField(m, "inputs", "source", "type")
	if typ != "directory" {
		t.Fatalf("expected directory, got %s", typ)
	}
}

func TestBuildJsonInput(t *testing.T) {
	b := NewManifestBuilder()
	ManifestInput[*JsonFile](b, "config")
	m := b.Build()

	typ, _ := getField(m, "inputs", "config", "type")
	if typ != "json" {
		t.Fatalf("expected json, got %s", typ)
	}
}

func TestBuildFileCollectionInput(t *testing.T) {
	b := NewManifestBuilder()
	ManifestInput[*FileCollection](b, "data")
	m := b.Build()

	typ, _ := getField(m, "inputs", "data", "type")
	itemType, _ := getField(m, "inputs", "data", "item_type")
	if typ != "collection" || itemType != "file" {
		t.Fatalf("expected collection/file, got %s/%s", typ, itemType)
	}
}
