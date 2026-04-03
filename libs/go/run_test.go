package spade

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRunNoOutput(t *testing.T) {
	dir := setupWorkDir(t)
	createInputFile(t, dir, "source", "data.tif", []byte("test data"))

	called := false
	err := RunNoOutputAt(dir, func(args *Args) error {
		_, err := Input[*File](args, "source")
		if err != nil {
			return err
		}
		called = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "outputs"))
	if len(entries) != 0 {
		t.Fatalf("expected empty outputs, got %d entries", len(entries))
	}
}

func TestRunWithParamsAndInputs(t *testing.T) {
	dir := setupWorkDir(t)
	writeParams(t, dir, "buffer: 30\nmethod: bilinear\n")
	createInputFile(t, dir, "raster", "data.tif", []byte("test data"))

	err := RunNoOutputAt(dir, func(args *Args) error {
		raster, err := Input[*RasterFile](args, "raster")
		if err != nil {
			return err
		}
		buffer, err := Param[int](args, "buffer")
		if err != nil {
			return err
		}
		method, err := Param[string](args, "method")
		if err != nil {
			return err
		}
		if filepath.Base(raster.Path) != "data.tif" {
			t.Fatalf("expected data.tif, got %s", filepath.Base(raster.Path))
		}
		if buffer != 30 {
			t.Fatalf("expected 30, got %d", buffer)
		}
		if method != "bilinear" {
			t.Fatalf("expected bilinear, got %s", method)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunReturningRasterFile(t *testing.T) {
	dir := setupWorkDir(t)
	createInputFile(t, dir, "source", "data.tif", []byte("test data"))

	resultPath := filepath.Join(dir, "processed.tif")
	os.WriteFile(resultPath, []byte("processed data"), 0644)

	err := RunAt(dir, func(args *Args) (*RasterFile, error) {
		_, err := Input[*RasterFile](args, "source")
		if err != nil {
			return nil, err
		}
		r := NewRasterFile(resultPath)
		return &r, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	files := walkDir(t, filepath.Join(dir, "outputs"))
	if len(files) != 1 {
		t.Fatalf("expected 1 output file, got %d", len(files))
	}
	data, _ := os.ReadFile(files[0])
	if string(data) != "processed data" {
		t.Fatalf("expected 'processed data', got %q", string(data))
	}
}

func TestRunReturningOutputs(t *testing.T) {
	dir := setupWorkDir(t)
	createInputFile(t, dir, "source", "data.tif", []byte("test data"))

	rasterPath := filepath.Join(dir, "result.tif")
	os.WriteFile(rasterPath, []byte("raster"), 0644)
	jsonPath := filepath.Join(dir, "stats.json")
	os.WriteFile(jsonPath, []byte(`{"mean": 42}`), 0644)

	err := RunAt(dir, func(args *Args) (*Outputs, error) {
		outputs := NewOutputs()
		r := NewRasterFile(rasterPath)
		j := NewJsonFile(jsonPath)
		outputs.Add("raster", &r)
		outputs.Add("stats", &j)
		return outputs, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "outputs", "raster", "result.tif")); err != nil {
		t.Fatal("raster output not found")
	}
	if _, err := os.Stat(filepath.Join(dir, "outputs", "stats", "stats.json")); err != nil {
		t.Fatal("stats output not found")
	}
}

func TestRunHandlerError(t *testing.T) {
	dir := setupWorkDir(t)
	createInputFile(t, dir, "source", "data.tif", []byte("test data"))

	err := RunNoOutputAt(dir, func(args *Args) error {
		return errors.New("processing failed")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "processing failed" {
		t.Fatalf("expected 'processing failed', got %q", err.Error())
	}
}

func TestRunFullWorkflow(t *testing.T) {
	dir := setupWorkDir(t)
	writeParams(t, dir, "resolution: 10\nmethod: nearest\n")
	createInputFile(t, dir, "reference", "ref.tif", []byte("reference raster data"))
	createInputFile(t, dir, "target", "tgt.tif", []byte("target raster data"))

	resultPath := filepath.Join(dir, "reprojected.tif")
	os.WriteFile(resultPath, []byte("reprojected output"), 0644)

	err := RunAt(dir, func(args *Args) (*RasterFile, error) {
		reference, err := Input[*RasterFile](args, "reference")
		if err != nil {
			return nil, err
		}
		target, err := Input[*RasterFile](args, "target")
		if err != nil {
			return nil, err
		}
		resolution, err := Param[int](args, "resolution")
		if err != nil {
			return nil, err
		}
		method, err := Param[string](args, "method")
		if err != nil {
			return nil, err
		}
		if filepath.Base(reference.Path) != "ref.tif" {
			t.Fatalf("unexpected reference path: %s", reference.Path)
		}
		if filepath.Base(target.Path) != "tgt.tif" {
			t.Fatalf("unexpected target path: %s", target.Path)
		}
		if resolution != 10 {
			t.Fatalf("expected resolution=10, got %d", resolution)
		}
		if method != "nearest" {
			t.Fatalf("expected method=nearest, got %s", method)
		}
		r := NewRasterFile(resultPath)
		return &r, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	files := walkDir(t, filepath.Join(dir, "outputs"))
	if len(files) != 1 {
		t.Fatalf("expected 1 output file, got %d", len(files))
	}
	data, _ := os.ReadFile(files[0])
	if string(data) != "reprojected output" {
		t.Fatalf("expected 'reprojected output', got %q", string(data))
	}
}

// walkDir recursively collects all file paths under a directory.
func walkDir(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	for _, entry := range entries {
		p := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			files = append(files, walkDir(t, p)...)
		} else {
			files = append(files, p)
		}
	}
	return files
}
