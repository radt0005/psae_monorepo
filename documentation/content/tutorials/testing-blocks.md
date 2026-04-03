+++
title = "Testing Blocks"
description = "Strategies for testing blocks during development."
weight = 4
+++

This tutorial covers strategies for testing Spade blocks at every stage of development. Testing blocks is straightforward because blocks have a simple, file-based interface: they read from `inputs/` and `params.yaml`, and write to `outputs/`. You can simulate this environment manually or use library-provided test utilities.

By the end of this tutorial you will know how to:

- Create a test working directory by hand
- Run a block handler directly outside of Spade
- Use the Go `RunAt` and Rust `run_at` test utilities
- Write unit tests for block logic
- Run integration tests using `spade run` with a single-block pipeline

## Strategy overview

There are three levels of testing for blocks, each catching different kinds of problems:

| Level | What it tests | Tools |
|-------|--------------|-------|
| **Manual run** | End-to-end handler logic with real data | Handmade working directory + direct execution |
| **Unit tests** | Core computation logic in isolation | Standard test frameworks (pytest, go test, cargo test) |
| **Integration test** | Full pipeline execution including sandbox and caching | `spade run` with a single-block pipeline |

Start with manual runs during early development to iterate quickly. Add unit tests once the logic stabilizes. Use integration tests to verify the block works correctly inside the Spade runtime (with sandboxing, input resolution, and output verification).

## Strategy 1: Manual test working directory

The simplest way to test a block is to create a working directory that looks like what Spade would create, then run the handler directly. This approach works for any language and does not require the Spade CLI.

### Create the directory structure

A block's working directory has this layout:

```
test-workdir/
  params.yaml
  inputs/
    <input-name>/
      <file>
  outputs/
  logs/
```

Create it manually:

```bash
mkdir -p test-workdir/inputs/red_band
mkdir -p test-workdir/inputs/nir_band
mkdir -p test-workdir/outputs
mkdir -p test-workdir/logs
```

### Provide test input files

Copy or symlink real data files into the input directories. Each input gets its own subdirectory under `inputs/`, named to match the input name in the block manifest:

```bash
cp /path/to/red_band.tif test-workdir/inputs/red_band/red_band.tif
cp /path/to/nir_band.tif test-workdir/inputs/nir_band/nir_band.tif
```

If you do not have real data, create small synthetic test files. For example, for the NDVI block from the [Building a Block](/tutorials/building-a-block/) tutorial:

```python
# create_test_data.py
import numpy as np
import rasterio
from rasterio.transform import from_bounds

# Create small 10x10 test rasters
transform = from_bounds(0, 0, 1, 1, 10, 10)
profile = {
    "driver": "GTiff",
    "dtype": "float64",
    "width": 10,
    "height": 10,
    "count": 1,
    "crs": "EPSG:4326",
    "transform": transform,
}

# Red band: uniform value of 0.2
red = np.full((1, 10, 10), 0.2)
with rasterio.open("test-workdir/inputs/red_band/red_band.tif", "w", **profile) as dst:
    dst.write(red)

# NIR band: uniform value of 0.8
nir = np.full((1, 10, 10), 0.8)
with rasterio.open("test-workdir/inputs/nir_band/nir_band.tif", "w", **profile) as dst:
    dst.write(nir)
```

With these values, the expected NDVI is `(0.8 - 0.2) / (0.8 + 0.2) = 0.6` for every pixel.

### Write params.yaml

Create `test-workdir/params.yaml` with the scalar parameters your block expects:

```yaml
nodata_value: -9999
```

This corresponds to what the pipeline's `args` would generate. Every key-value pair in `args` ends up in `params.yaml`.

### Run the handler

Change into the test working directory and run the handler script directly:

```bash
cd test-workdir
python ../src/raster_tools/ndvi.py
```

The handler reads `params.yaml` and `inputs/`, computes the result, and writes to `outputs/`. After it finishes, check the output:

```bash
ls outputs/ndvi_raster/
# ndvi.tif
```

You can inspect the output file with any raster tool:

```python
import rasterio
import numpy as np

with rasterio.open("outputs/ndvi_raster/ndvi.tif") as src:
    ndvi = src.read(1)
    print(f"Min: {ndvi.min():.2f}, Max: {ndvi.max():.2f}, Mean: {ndvi.mean():.2f}")
    # Expected: Min: 0.60, Max: 0.60, Mean: 0.60
```

### Testing other languages

The same approach works for any language. Just set up the same directory structure and run the block's entrypoint:

**Go:**
```bash
cd test-workdir
go run ../cmd/ndvi/main.go
```

**Rust:**
```bash
cd test-workdir
cargo run --bin ndvi
```

**R:**
```bash
cd test-workdir
Rscript ../R/ndvi.R
```

**TypeScript:**
```bash
cd test-workdir
bun run ../src/ndvi.ts
```

## Strategy 2: Library test utilities

The Go and Rust Spade libraries provide helper functions that let you run a handler at a specific base path. This makes it easy to write automated tests that set up a temporary working directory, run the handler, and check the results -- all within your standard test framework.

### Go: RunAt and RunNoOutputAt

The Go library provides `RunAt` and `RunNoOutputAt`, which are test-friendly versions of `Run` and `RunNoOutput`. Instead of running in the current directory, they accept a base path pointing to a test working directory:

```go
package myblock

import (
    "os"
    "path/filepath"
    "testing"

    spade "github.com/spade-dev/spade"
)

func TestNDVIHandler(t *testing.T) {
    // Create a temporary working directory
    dir := t.TempDir()
    for _, sub := range []string{"inputs", "outputs", "logs"} {
        os.MkdirAll(filepath.Join(dir, sub), 0755)
    }

    // Write params.yaml
    os.WriteFile(
        filepath.Join(dir, "params.yaml"),
        []byte("nodata_value: -9999\n"),
        0644,
    )

    // Create test input files
    redDir := filepath.Join(dir, "inputs", "red_band")
    os.MkdirAll(redDir, 0755)
    os.WriteFile(filepath.Join(redDir, "red.tif"), testRedData, 0644)

    nirDir := filepath.Join(dir, "inputs", "nir_band")
    os.MkdirAll(nirDir, 0755)
    os.WriteFile(filepath.Join(nirDir, "nir.tif"), testNirData, 0644)

    // Run the handler at the test directory
    err := spade.RunAt(dir, handler)
    if err != nil {
        t.Fatalf("handler failed: %v", err)
    }

    // Verify the output exists
    outputPath := filepath.Join(dir, "outputs", "ndvi_raster", "ndvi.tif")
    if _, err := os.Stat(outputPath); err != nil {
        t.Fatalf("expected output file at %s, got error: %v", outputPath, err)
    }

    // Read and verify the output
    data, _ := os.ReadFile(outputPath)
    // ... verify NDVI values ...
}
```

Key points:

- **`t.TempDir()`** creates a temporary directory that is automatically cleaned up when the test finishes.
- **`spade.RunAt(dir, handler)`** runs the handler function as if the working directory were `dir`. It loads inputs from `dir/inputs/`, reads parameters from `dir/params.yaml`, and writes outputs to `dir/outputs/`.
- The test verifies both that the handler succeeds (no error) and that the expected output file was created.

For handlers that produce no output (side-effect-only blocks), use `RunNoOutputAt`:

```go
err := spade.RunNoOutputAt(dir, func(args *spade.Args) error {
    // Block logic that does not return an output
    return nil
})
```

### Rust: run_at

The Rust library provides a `run_at` function (used internally in tests) that works similarly. Here is how to use it in a test:

```rust
#[cfg(test)]
mod tests {
    use spade::{Args, RasterFile};
    use std::fs;
    use tempfile::TempDir;

    // Helper to create the working directory structure
    fn setup_work_dir() -> TempDir {
        let dir = TempDir::new().unwrap();
        fs::create_dir_all(dir.path().join("inputs")).unwrap();
        fs::create_dir_all(dir.path().join("outputs")).unwrap();
        fs::create_dir_all(dir.path().join("logs")).unwrap();
        dir
    }

    fn write_params(dir: &TempDir, content: &str) {
        fs::write(dir.path().join("params.yaml"), content).unwrap();
    }

    fn create_input_file(dir: &TempDir, name: &str, filename: &str, content: &[u8]) {
        let input_dir = dir.path().join("inputs").join(name);
        fs::create_dir_all(&input_dir).unwrap();
        fs::write(input_dir.join(filename), content).unwrap();
    }

    #[test]
    fn test_ndvi_handler() {
        let dir = setup_work_dir();
        write_params(&dir, "nodata_value: -9999\n");
        create_input_file(&dir, "red_band", "red.tif", &test_red_data());
        create_input_file(&dir, "nir_band", "nir.tif", &test_nir_data());

        // run_at executes the handler at the given base path
        spade::run_at(
            dir.path(),
            |args: Args| -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
                let red: RasterFile = args.input("red_band")?;
                let nir: RasterFile = args.input("nir_band")?;
                let nodata: f64 = args.param("nodata_value")?;

                // ... compute NDVI ...

                Ok(RasterFile::new("outputs/ndvi_raster/ndvi.tif"))
            },
        )
        .unwrap();

        // Verify output
        assert!(dir.path().join("outputs/ndvi_raster/ndvi.tif").exists());
    }
}
```

The pattern is the same as Go:

1. Create a temporary directory with `inputs/`, `outputs/`, `logs/`
2. Write `params.yaml` and input files
3. Call `run_at` with the directory and handler
4. Assert the handler succeeded and check the outputs

### Python: manual setup

The Python library does not currently expose a `run_at` function, but you can achieve the same thing by changing directories:

```python
import os
import tempfile
import shutil

def test_ndvi_handler():
    # Create a temporary working directory
    workdir = tempfile.mkdtemp()
    os.makedirs(os.path.join(workdir, "inputs", "red_band"))
    os.makedirs(os.path.join(workdir, "inputs", "nir_band"))
    os.makedirs(os.path.join(workdir, "outputs"))
    os.makedirs(os.path.join(workdir, "logs"))

    # Write params.yaml
    with open(os.path.join(workdir, "params.yaml"), "w") as f:
        f.write("nodata_value: -9999\n")

    # Copy test input files
    shutil.copy("tests/fixtures/red_band.tif",
                os.path.join(workdir, "inputs", "red_band", "red_band.tif"))
    shutil.copy("tests/fixtures/nir_band.tif",
                os.path.join(workdir, "inputs", "nir_band", "nir_band.tif"))

    # Change to the working directory and run the handler
    original_dir = os.getcwd()
    try:
        os.chdir(workdir)
        from raster_tools.ndvi import handler
        from spade import RasterFile

        result = handler(
            red_band=RasterFile(path="inputs/red_band/red_band.tif"),
            nir_band=RasterFile(path="inputs/nir_band/nir_band.tif"),
            nodata_value=-9999,
        )

        # Verify the output
        assert os.path.exists("outputs/ndvi_raster/ndvi.tif")
    finally:
        os.chdir(original_dir)
        shutil.rmtree(workdir)
```

Alternatively, you can call the handler function directly without changing directories, passing the constructed typed arguments. This is the approach shown above -- calling `handler()` with pre-built arguments. The `run()` function is the one that reads from the filesystem, but the handler itself just takes typed arguments and writes to paths.

## Strategy 3: Unit testing the core logic

For complex blocks, separate the core computation from the Spade I/O layer. This makes the computation testable with standard unit tests, without needing to set up a working directory at all.

### Separate computation from I/O

Refactor the handler so the core logic is in a pure function:

```python
# raster_tools/ndvi.py
import numpy as np
import rasterio
from spade import run, RasterFile


def compute_ndvi(red: np.ndarray, nir: np.ndarray, nodata: float) -> np.ndarray:
    """Pure computation: NDVI = (NIR - Red) / (NIR + Red).

    This function has no file I/O -- it takes arrays in and returns an array.
    """
    denominator = nir + red
    return np.where(denominator != 0, (nir - red) / denominator, nodata)


def handler(red_band: RasterFile, nir_band: RasterFile, nodata_value: float) -> RasterFile:
    """Spade handler: reads files, calls compute_ndvi, writes output."""
    with rasterio.open(red_band.path) as src:
        red = src.read(1).astype(np.float64)
        profile = src.profile.copy()

    with rasterio.open(nir_band.path) as src:
        nir = src.read(1).astype(np.float64)

    ndvi = compute_ndvi(red, nir, nodata_value)

    profile.update(dtype=rasterio.float64, count=1, nodata=nodata_value)
    output_path = "outputs/ndvi_raster/ndvi.tif"
    with rasterio.open(output_path, "w", **profile) as dst:
        dst.write(ndvi, 1)

    return RasterFile(path=output_path)


if __name__ == "__main__":
    run(handler)
```

### Test the pure function

Now you can test `compute_ndvi` without any files:

```python
# tests/test_ndvi.py
import numpy as np
from raster_tools.ndvi import compute_ndvi


def test_ndvi_basic():
    """NDVI of red=0.2, nir=0.8 should be 0.6."""
    red = np.array([[0.2, 0.2], [0.2, 0.2]])
    nir = np.array([[0.8, 0.8], [0.8, 0.8]])
    result = compute_ndvi(red, nir, nodata=-9999)
    np.testing.assert_allclose(result, 0.6)


def test_ndvi_zero_denominator():
    """Pixels where both bands are zero should get the nodata value."""
    red = np.array([[0.0, 0.2], [0.0, 0.5]])
    nir = np.array([[0.0, 0.8], [0.0, 0.5]])
    result = compute_ndvi(red, nir, nodata=-9999)
    assert result[0, 0] == -9999
    assert result[1, 0] == -9999
    np.testing.assert_allclose(result[0, 1], 0.6)
    np.testing.assert_allclose(result[1, 1], 0.0)


def test_ndvi_range():
    """NDVI values should be between -1 and 1 (excluding nodata)."""
    rng = np.random.default_rng(42)
    red = rng.uniform(0.01, 1.0, size=(100, 100))
    nir = rng.uniform(0.01, 1.0, size=(100, 100))
    result = compute_ndvi(red, nir, nodata=-9999)
    valid = result[result != -9999]
    assert valid.min() >= -1.0
    assert valid.max() <= 1.0


def test_ndvi_perfect_vegetation():
    """When NIR >> Red, NDVI should be close to 1."""
    red = np.array([[0.01]])
    nir = np.array([[0.99]])
    result = compute_ndvi(red, nir, nodata=-9999)
    assert result[0, 0] > 0.95
```

Run the tests with your normal test runner:

```bash
pytest tests/test_ndvi.py -v
```

This pattern applies to any language. In Go:

```go
func TestComputeNDVI(t *testing.T) {
    red := 0.2
    nir := 0.8
    expected := 0.6
    result := computeNDVI(red, nir, -9999)
    if math.Abs(result-expected) > 1e-6 {
        t.Fatalf("expected %f, got %f", expected, result)
    }
}
```

In Rust:

```rust
#[test]
fn test_compute_ndvi() {
    let result = compute_ndvi(0.2, 0.8, -9999.0);
    assert!((result - 0.6).abs() < 1e-6);
}
```

## Strategy 4: Integration testing with spade run

Once you are confident the block logic is correct, test it inside the full Spade runtime. This catches issues that unit tests miss, such as:

- Sandbox restrictions preventing file access
- Missing output files that Spade expects
- Incorrect file paths relative to the working directory
- Dependency resolution and caching behavior

### Create a single-block test pipeline

Write a minimal pipeline that exercises just your block:

```yaml
# test-ndvi-pipeline.yaml
id: 019d4000-0000-7000-0000-000000000000
name: test-ndvi
version: "1.0"
description: Integration test for the NDVI block

blocks:
  # Use a data source block to provide input files
  - id: 019d4000-0001-7000-0000-000000000000
    name: data.local-file
    inputs: []
    args:
      path: "/path/to/test/red_band.tif"

  - id: 019d4000-0002-7000-0000-000000000000
    name: data.local-file
    inputs: []
    args:
      path: "/path/to/test/nir_band.tif"

  - id: 019d4000-0003-7000-0000-000000000000
    name: raster-tools.ndvi
    inputs:
      - block: 019d4000-0001-7000-0000-000000000000
        output: file
        as: red_band
      - block: 019d4000-0002-7000-0000-000000000000
        output: file
        as: nir_band
    args:
      nodata_value: -9999
```

### Install and run

Make sure your block is installed:

```bash
cd raster-tools
spade install file://.
```

Validate the test pipeline:

```bash
spade check test-ndvi-pipeline.yaml
```

Run it:

```bash
spade run --keep-work-dir test-ndvi-pipeline.yaml
```

The `--keep-work-dir` flag preserves the working directory so you can inspect the outputs afterward.

### Inspect the results

After the pipeline runs, check the block's working directory:

```bash
# Find the working directory
ls ~/.spade/pipelines/019d4000-0000-7000-0000-000000000000/

# Check the NDVI block's outputs
ls ~/.spade/pipelines/019d4000-0000-7000-0000-000000000000/019d4000-0003-7000-0000-000000000000/outputs/

# Check logs for any warnings
cat ~/.spade/pipelines/019d4000-0000-7000-0000-000000000000/019d4000-0003-7000-0000-000000000000/logs/stderr.log
```

### Debugging failures

If the block fails, the error message tells you which block failed and its exit code:

```
Block raster-tools.ndvi failed: process exited with status 1
```

Check the block's logs:

```bash
cat ~/.spade/pipelines/<run-id>/<block-id>/logs/stderr.log
```

Common causes of integration test failures:

- **File not found errors** -- The block is trying to read a file outside its sandbox. All file access must go through `inputs/` and `outputs/`.
- **Missing output** -- The block did not write all declared outputs to the `outputs/` directory. Check that the output directory names match the manifest exactly.
- **Permission denied** -- The `isolate` sandbox restricts filesystem access. Make sure the block only reads from `inputs/` and writes to `outputs/`.
- **Import errors** -- A Python dependency is not installed in the block's environment. Make sure it is listed in `pyproject.toml` and the collection was reinstalled with `spade install file://.`.

## Testing map and reduce blocks

Map and reduce blocks can be tested using the same strategies, with slight differences in the test setup.

### Testing a map block

A map block writes an expansion manifest. To test it manually, set up the working directory with the block's inputs, run the handler, and verify that both the data files and the `expansion.yaml` manifest were written correctly:

```bash
# After running the map block handler
cat test-workdir/outputs/tile/expansion.yaml
```

Verify the manifest lists the correct number of items and that each referenced file exists.

### Testing a reduce block

A reduce block reads a collection input. To set up the test working directory, create numbered files in the input directory:

```bash
mkdir -p test-workdir/inputs/tiles
cp tile_0.tif test-workdir/inputs/tiles/001.tif
cp tile_1.tif test-workdir/inputs/tiles/002.tif
cp tile_2.tif test-workdir/inputs/tiles/003.tif
```

The zero-padded numbering (`001`, `002`, `003`) matches what Spade produces when it collects outputs from parallel invocations. The reduce block's handler receives these as a collection (for example, `RasterFileCollection` in Python with a `.paths` list).

## Testing checklist

Here is a checklist to follow when testing a new block:

1. **Create synthetic test data** that exercises the block's logic, including edge cases (empty inputs, zero values, very large values).

2. **Test the pure computation** with unit tests. No files needed -- just pass arrays or values in and check the results.

3. **Test the handler** with a manual working directory. Verify that the correct output files are created and contain valid data.

4. **Validate the manifest** with `spade check` in the collection directory. This catches structural errors before you try to run anything.

5. **Install and run** an integration test pipeline with `spade run --keep-work-dir`. This verifies the block works inside the Spade runtime with sandboxing.

6. **Check edge cases** in integration: What happens if an input file is empty? What if a parameter is at its boundary value? Does the block report errors correctly (non-zero exit code) rather than producing silently wrong output?

## Next steps

- Learn how to [build a block](/tutorials/building-a-block/) from scratch with testing in mind
- Read about [writing pipelines](/tutorials/writing-pipelines/) to connect tested blocks together
- See the library documentation for [Python](/libraries/python/), [Go](/libraries/go/), and [Rust](/libraries/rust/) for the full API reference
