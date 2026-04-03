package spade

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func setupWorkDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"inputs", "outputs", "logs"} {
		os.MkdirAll(filepath.Join(dir, sub), 0755)
	}
	return dir
}

func writeParams(t *testing.T, dir, content string) {
	t.Helper()
	os.WriteFile(filepath.Join(dir, "params.yaml"), []byte(content), 0644)
}

func createInputFile(t *testing.T, dir, name, filename string, content []byte) {
	t.Helper()
	inputDir := filepath.Join(dir, "inputs", name)
	os.MkdirAll(inputDir, 0755)
	os.WriteFile(filepath.Join(inputDir, filename), content, 0644)
}

// ---------------------------------------------------------------------------
// LoadParams tests
// ---------------------------------------------------------------------------

func TestLoadParamsBasic(t *testing.T) {
	dir := setupWorkDir(t)
	writeParams(t, dir, "buffer_distance: 30\nmethod: bilinear\n")
	params, err := LoadParams(dir)
	if err != nil {
		t.Fatal(err)
	}
	if params["buffer_distance"] != 30 {
		t.Fatalf("expected buffer_distance=30, got %v", params["buffer_distance"])
	}
	if params["method"] != "bilinear" {
		t.Fatalf("expected method=bilinear, got %v", params["method"])
	}
}

func TestLoadParamsEmptyFile(t *testing.T) {
	dir := setupWorkDir(t)
	writeParams(t, dir, "")
	params, err := LoadParams(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 0 {
		t.Fatalf("expected empty map, got %v", params)
	}
}

func TestLoadParamsMissingFile(t *testing.T) {
	dir := setupWorkDir(t)
	params, err := LoadParams(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 0 {
		t.Fatalf("expected empty map, got %v", params)
	}
}

func TestLoadParamsTypes(t *testing.T) {
	dir := setupWorkDir(t)
	writeParams(t, dir, "name: hello\ncount: 42\nrate: 3.14\nenabled: true\n")
	params, err := LoadParams(dir)
	if err != nil {
		t.Fatal(err)
	}
	if params["name"] != "hello" {
		t.Fatalf("expected name=hello, got %v", params["name"])
	}
	if params["count"] != 42 {
		t.Fatalf("expected count=42, got %v (%T)", params["count"], params["count"])
	}
	if params["enabled"] != true {
		t.Fatalf("expected enabled=true, got %v", params["enabled"])
	}
}

// ---------------------------------------------------------------------------
// ScanInputs tests
// ---------------------------------------------------------------------------

func TestScanInputsSingleFile(t *testing.T) {
	dir := setupWorkDir(t)
	createInputFile(t, dir, "raster", "data.tif", []byte("test data"))
	inputs, err := ScanInputs(dir)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := inputs["raster"]
	if !ok {
		t.Fatal("expected 'raster' input")
	}
	if entry.Kind != InputSingle {
		t.Fatalf("expected InputSingle, got %d", entry.Kind)
	}
	if !filepath.IsAbs(entry.Path) || filepath.Base(entry.Path) != "data.tif" {
		t.Fatalf("unexpected path: %s", entry.Path)
	}
}

func TestScanInputsMultipleFiles(t *testing.T) {
	dir := setupWorkDir(t)
	createInputFile(t, dir, "tiles", "001.tif", []byte("data"))
	createInputFile(t, dir, "tiles", "002.tif", []byte("data"))
	createInputFile(t, dir, "tiles", "003.tif", []byte("data"))
	inputs, err := ScanInputs(dir)
	if err != nil {
		t.Fatal(err)
	}
	entry := inputs["tiles"]
	if entry.Kind != InputMultiple {
		t.Fatalf("expected InputMultiple, got %d", entry.Kind)
	}
	if len(entry.Paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(entry.Paths))
	}
}

func TestScanInputsEmptyDirError(t *testing.T) {
	dir := setupWorkDir(t)
	os.MkdirAll(filepath.Join(dir, "inputs", "empty"), 0755)
	_, err := ScanInputs(dir)
	if err == nil {
		t.Fatal("expected error")
	}
	var emptyErr *ErrEmptyInputDir
	if !errors.As(err, &emptyErr) {
		t.Fatalf("expected ErrEmptyInputDir, got %T: %v", err, err)
	}
}

func TestScanInputsNoInputsDir(t *testing.T) {
	dir := t.TempDir()
	inputs, err := ScanInputs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 0 {
		t.Fatalf("expected empty map, got %v", inputs)
	}
}

func TestScanInputsMultipleSubdirs(t *testing.T) {
	dir := setupWorkDir(t)
	createInputFile(t, dir, "reference", "ref.tif", []byte("ref"))
	createInputFile(t, dir, "target", "tgt.tif", []byte("tgt"))
	inputs, err := ScanInputs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := inputs["reference"]; !ok {
		t.Fatal("expected 'reference' input")
	}
	if _, ok := inputs["target"]; !ok {
		t.Fatal("expected 'target' input")
	}
}

// ---------------------------------------------------------------------------
// Input[T] tests
// ---------------------------------------------------------------------------

func TestInputRasterFile(t *testing.T) {
	dir := setupWorkDir(t)
	createInputFile(t, dir, "source", "data.tif", []byte("test"))
	args, err := BuildArgs(dir)
	if err != nil {
		t.Fatal(err)
	}
	raster, err := Input[*RasterFile](args, "source")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(raster.Path) != "data.tif" {
		t.Fatalf("expected data.tif, got %s", filepath.Base(raster.Path))
	}
}

func TestInputRasterFileCollection(t *testing.T) {
	dir := setupWorkDir(t)
	createInputFile(t, dir, "tiles", "a.tif", []byte("a"))
	createInputFile(t, dir, "tiles", "b.tif", []byte("b"))
	args, err := BuildArgs(dir)
	if err != nil {
		t.Fatal(err)
	}
	coll, err := Input[*RasterFileCollection](args, "tiles")
	if err != nil {
		t.Fatal(err)
	}
	if len(coll.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(coll.Paths))
	}
}

func TestInputNotFound(t *testing.T) {
	dir := setupWorkDir(t)
	args, err := BuildArgs(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Input[*File](args, "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	var notFound *ErrInputNotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("expected ErrInputNotFound, got %T", err)
	}
}

// ---------------------------------------------------------------------------
// Param[T] tests
// ---------------------------------------------------------------------------

func TestParamInt(t *testing.T) {
	dir := setupWorkDir(t)
	writeParams(t, dir, "resolution: 10\n")
	args, err := BuildArgs(dir)
	if err != nil {
		t.Fatal(err)
	}
	val, err := Param[int](args, "resolution")
	if err != nil {
		t.Fatal(err)
	}
	if val != 10 {
		t.Fatalf("expected 10, got %d", val)
	}
}

func TestParamFloat64(t *testing.T) {
	dir := setupWorkDir(t)
	writeParams(t, dir, "resolution: 10.5\n")
	args, err := BuildArgs(dir)
	if err != nil {
		t.Fatal(err)
	}
	val, err := Param[float64](args, "resolution")
	if err != nil {
		t.Fatal(err)
	}
	if val != 10.5 {
		t.Fatalf("expected 10.5, got %f", val)
	}
}

func TestParamString(t *testing.T) {
	dir := setupWorkDir(t)
	writeParams(t, dir, "method: bilinear\n")
	args, err := BuildArgs(dir)
	if err != nil {
		t.Fatal(err)
	}
	val, err := Param[string](args, "method")
	if err != nil {
		t.Fatal(err)
	}
	if val != "bilinear" {
		t.Fatalf("expected bilinear, got %s", val)
	}
}

func TestParamBool(t *testing.T) {
	dir := setupWorkDir(t)
	writeParams(t, dir, "normalize: true\n")
	args, err := BuildArgs(dir)
	if err != nil {
		t.Fatal(err)
	}
	val, err := Param[bool](args, "normalize")
	if err != nil {
		t.Fatal(err)
	}
	if !val {
		t.Fatal("expected true")
	}
}

func TestParamNotFound(t *testing.T) {
	dir := setupWorkDir(t)
	args, err := BuildArgs(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Param[float64](args, "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	var notFound *ErrParamNotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("expected ErrParamNotFound, got %T", err)
	}
}

func TestParamIntAsFloat(t *testing.T) {
	dir := setupWorkDir(t)
	writeParams(t, dir, "count: 42\n")
	args, err := BuildArgs(dir)
	if err != nil {
		t.Fatal(err)
	}
	// YAML parses 42 as int, but we want float64
	val, err := Param[float64](args, "count")
	if err != nil {
		t.Fatal(err)
	}
	if val != 42.0 {
		t.Fatalf("expected 42.0, got %f", val)
	}
}

// ---------------------------------------------------------------------------
// HasInput / HasParam tests
// ---------------------------------------------------------------------------

func TestHasInput(t *testing.T) {
	dir := setupWorkDir(t)
	createInputFile(t, dir, "source", "data.tif", []byte("test"))
	args, err := BuildArgs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !args.HasInput("source") {
		t.Fatal("expected HasInput('source') = true")
	}
	if args.HasInput("missing") {
		t.Fatal("expected HasInput('missing') = false")
	}
}

func TestHasParam(t *testing.T) {
	dir := setupWorkDir(t)
	writeParams(t, dir, "buffer: 30\n")
	args, err := BuildArgs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !args.HasParam("buffer") {
		t.Fatal("expected HasParam('buffer') = true")
	}
	if args.HasParam("missing") {
		t.Fatal("expected HasParam('missing') = false")
	}
}

func TestBuildArgsParamsAndInputs(t *testing.T) {
	dir := setupWorkDir(t)
	writeParams(t, dir, "buffer: 30\n")
	createInputFile(t, dir, "raster", "data.tif", []byte("test"))
	args, err := BuildArgs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !args.HasParam("buffer") {
		t.Fatal("expected buffer param")
	}
	if !args.HasInput("raster") {
		t.Fatal("expected raster input")
	}
}
