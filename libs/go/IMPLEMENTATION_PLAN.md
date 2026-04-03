# Go Spade Library Implementation Plan

This plan covers the full implementation of the Go runtime library for building Spade blocks. The library mirrors the functionality of the existing Python and Rust reference implementations, adapted to idiomatic Go patterns. It uses Go generics (1.18+), interfaces, and struct embedding to achieve a clean API.

## Module Structure

```
spade/
  go.mod
  types.go          — Type definitions (File, Directory, collections, interfaces)
  types_test.go     — Tests for types
  scanning.go       — Load params.yaml, scan inputs/, build Args
  scanning_test.go  — Tests for scanning
  output.go         — Write outputs, read block manifest, Outputs collection
  output_test.go    — Tests for output
  run.go            — Main run() entry point
  run_test.go       — Tests for run
  build.go          — ManifestBuilder for generating block manifests
  build_test.go     — Tests for build
  errors.go         — Custom error types
```

---

## Phase 1: Error Types (`errors.go`)

- [x] Define a `SpadeError` interface or use sentinel error types for structured errors. [DONE]
- [x] Implement `ErrInputNotFound` — returned when `Args.Input(name)` references a name not present in `inputs/`. Contains the missing input name. [DONE]
- [x] Implement `ErrParamNotFound` — returned when `Args.Param(name)` references a name not present in `params.yaml`. Contains the missing param name. [DONE]
- [x] Implement `ErrEmptyInputDir` — returned when an input subdirectory exists but contains no files. Contains the directory name. [DONE]
- [x] Implement `ErrTypeMismatch` — returned when an input value cannot be converted to the requested type (e.g., requesting `Directory` from a file input). Contains name, expected type string, and found type string. [DONE]
- [x] All error types implement the `error` interface with descriptive `Error()` messages matching the format: `"input not found: 'name'"`, `"parameter not found: 'name'"`, `"input directory 'name' is empty"`, `"type mismatch for 'name': expected X, found Y"`. [DONE]

## Phase 2: Type System (`types.go`, `types_test.go`)

### 2.1 Interfaces

- [x] Define `SpadeType` interface with methods:
  - `TypeName() string` — the Spade type string (e.g., `"file"`, `"directory"`, `"collection"`).
  - `DefaultOutputName() string` — the default output subdirectory name (e.g., `"raster"`, `"files"`).
  - `ManifestEntry() ManifestInfo` — returns metadata for manifest generation.
- [x] Define `FromInput` interface with methods:
  - `FromSingleFile(path string) error` — populate the receiver from a single file path.
  - `FromMultipleFiles(paths []string) error` — populate the receiver from multiple file paths.
  - `FromDirectory(path string) error` — populate the receiver from a directory path.
- [x] Define `IntoOutput` interface with methods:
  - `WriteTo(outputDir string) error` — write the value to the given output directory.
  - `DefaultOutputName() string` — return the default output directory name.
- [x] Define `ManifestInfo` struct with fields: `TypeName string`, `Format string` (empty string = not set), `ItemType string` (empty string = not set).

### 2.2 Single-File Types

Each of these types is a struct with a `Path string` field. Each implements `SpadeType`, `FromInput`, and `IntoOutput`.

- [x] `File` — generic file type. `TypeName()` returns `"file"`, `DefaultOutputName()` returns `"file"`, `ManifestEntry()` returns `{TypeName: "file"}`. `FromSingleFile` sets `Path`. `FromMultipleFiles` takes the first path. `FromDirectory` returns `ErrTypeMismatch`. `WriteTo` copies the file to `outputDir/<filename>`, creating `outputDir` with `os.MkdirAll`.
- [x] `RasterFile` — raster data. Same as `File` but `DefaultOutputName()` returns `"raster"`, `ManifestEntry()` returns `{TypeName: "file", Format: "GeoTIFF"}`.
- [x] `VectorFile` — vector data. `DefaultOutputName()` returns `"vector"`, `ManifestEntry()` returns `{TypeName: "file", Format: "GeoJSON"}`.
- [x] `TabularFile` — tabular data. `DefaultOutputName()` returns `"tabular"`, `ManifestEntry()` returns `{TypeName: "file", Format: "CSV"}`.
- [x] `JsonFile` — JSON data. `TypeName()` returns `"json"`, `DefaultOutputName()` returns `"json"`, `ManifestEntry()` returns `{TypeName: "json"}`.
- [x] Provide a `NewFile(path string) File` constructor for each type (e.g., `NewRasterFile`, `NewVectorFile`, etc.) that returns the struct with `Path` set.

### 2.3 Directory Type

- [x] `Directory` struct with `Path string` field. Implements `SpadeType`, `FromInput`, `IntoOutput`.
- [x] `TypeName()` returns `"directory"`, `DefaultOutputName()` returns `"directory"`.
- [x] `FromSingleFile` and `FromMultipleFiles` return `ErrTypeMismatch`.
- [x] `FromDirectory` sets `Path`.
- [x] `WriteTo` creates `outputDir` with `os.MkdirAll`, then recursively copies all contents (files and subdirectories) from `Path` into `outputDir`.
- [x] Implement `copyDirRecursive(src, dst string) error` helper (unexported) that walks the source directory and copies files/directories into dst.

### 2.4 Collection Types

Each has a `Paths []string` field. Each implements `SpadeType`, `FromInput`, `IntoOutput`.

- [x] `FileCollection` — `TypeName()` returns `"collection"`, `DefaultOutputName()` returns `"files"`, `ManifestEntry()` returns `{TypeName: "collection", ItemType: "file"}`. `FromSingleFile` creates a single-element slice. `FromMultipleFiles` sets `Paths`. `FromDirectory` returns `ErrTypeMismatch`. `WriteTo` copies each file to `outputDir/<filename>`.
- [x] `RasterFileCollection` — same behavior, `DefaultOutputName()` returns `"rasters"`, `ManifestEntry()` returns `{TypeName: "collection", Format: "GeoTIFF", ItemType: "file"}`.
- [x] `VectorFileCollection` — `DefaultOutputName()` returns `"vectors"`, `ManifestEntry()` returns `{TypeName: "collection", Format: "GeoJSON", ItemType: "file"}`.
- [x] `TabularFileCollection` — `DefaultOutputName()` returns `"tables"`, `ManifestEntry()` returns `{TypeName: "collection", Format: "CSV", ItemType: "file"}`.
- [x] Provide `NewFileCollection(paths []string)`, `NewRasterFileCollection(paths []string)`, etc. constructors.

### 2.5 Tests

- [x] Test construction of each type via `New*` constructors and verify field values.
- [x] Test `TypeName()`, `DefaultOutputName()`, and `ManifestEntry()` return correct values for every type.
- [x] Test `FromSingleFile`, `FromMultipleFiles`, `FromDirectory` for each type — verify success cases set fields correctly and error cases return `ErrTypeMismatch`.
- [x] Test `WriteTo` for single-file types: create a temp file, call `WriteTo` into a temp output dir, verify the file is copied with the correct filename.
- [x] Test `WriteTo` for `Directory`: create a temp directory with files and subdirectories, call `WriteTo`, verify recursive copy.
- [x] Test `WriteTo` for collection types: create multiple temp files, call `WriteTo`, verify all files are copied.
- [x] Test clone/equality behavior (compare struct values).

## Phase 3: Input Scanning (`scanning.go`, `scanning_test.go`)

### 3.1 Parameter Loading

- [x] Implement `LoadParams(base string) (map[string]any, error)` — reads `<base>/params.yaml`, parses with `gopkg.in/yaml.v3` into `map[string]any`. Returns empty map (not error) if file does not exist. Returns empty map if file is empty/null.
- [x] Add `gopkg.in/yaml.v3` as a dependency in `go.mod`.

### 3.2 Input Scanning

- [x] Define `InputEntry` as a union-like type: either `SingleFile` (one path) or `MultipleFiles` (slice of paths). Use a struct with a `Kind` enum (`InputSingle`, `InputMultiple`) and `Path string` / `Paths []string` fields.
- [x] Implement `ScanInputs(base string) (map[string]InputEntry, error)` — reads `<base>/inputs/` directory, iterates sorted subdirectories. For each subdirectory:
  - Lists files (not subdirectories) in the subdirectory, sorted.
  - If no files found, returns `ErrEmptyInputDir`.
  - If exactly 1 file, creates `InputEntry` with `Kind=InputSingle`, `Path=<file path>`.
  - If multiple files, creates `InputEntry` with `Kind=InputMultiple`, `Paths=<sorted file paths>`.
  - Maps subdirectory name to the `InputEntry`.
- [x] Returns empty map (not error) if `inputs/` directory does not exist.

### 3.3 Args Struct

- [x] Define `Args` struct with unexported fields: `params map[string]any`, `inputs map[string]InputEntry`.
- [x] Implement `Input[T FromInput](args *Args, name string) (T, error)` as a generic free function — retrieves the `InputEntry` by name from `args.inputs`. If not found, returns zero value and `ErrInputNotFound`. Calls `T.FromSingleFile` or `T.FromMultipleFiles` based on `InputEntry.Kind`. This is a generic function because Go methods cannot have type parameters.
- [x] Implement `Param[T any](args *Args, name string) (T, error)` as a generic free function — retrieves the raw `any` value by name from `args.params`. If not found, returns zero value and `ErrParamNotFound`. Converts the value to `T` using YAML-compatible type assertion/conversion:
  - `string` → direct assertion
  - `int`, `int64`, `float64` → numeric conversion handling YAML's tendency to parse integers as `int` and floats as `float64`
  - `bool` → direct assertion
  - For other types, attempt `yaml.Marshal` then `yaml.Unmarshal` into `T` as a fallback.
- [x] Implement `(args *Args) HasInput(name string) bool` — checks if the input exists.
- [x] Implement `(args *Args) HasParam(name string) bool` — checks if the param exists.
- [x] Implement `BuildArgs(base string) (*Args, error)` — calls `LoadParams` and `ScanInputs`, returns a new `Args`.

### 3.4 Tests

- [x] Test `LoadParams` with a basic YAML file containing string, integer, float, and boolean values.
- [x] Test `LoadParams` with an empty file (returns empty map).
- [x] Test `LoadParams` with a missing file (returns empty map, no error).
- [x] Test `ScanInputs` with a single file in a subdirectory (returns `InputSingle`).
- [x] Test `ScanInputs` with multiple files in a subdirectory (returns `InputMultiple` with sorted paths).
- [x] Test `ScanInputs` with an empty subdirectory (returns `ErrEmptyInputDir`).
- [x] Test `ScanInputs` with no `inputs/` directory (returns empty map, no error).
- [x] Test `ScanInputs` with multiple subdirectories.
- [x] Test `Input[RasterFile]` retrieves a single file correctly.
- [x] Test `Input[RasterFileCollection]` retrieves multiple files correctly.
- [x] Test `Input` with a missing name returns `ErrInputNotFound`.
- [x] Test `Param[int64]` retrieves an integer parameter.
- [x] Test `Param[float64]` retrieves a float parameter.
- [x] Test `Param[string]` retrieves a string parameter.
- [x] Test `Param[bool]` retrieves a boolean parameter.
- [x] Test `Param` with a missing name returns `ErrParamNotFound`.
- [x] Test `HasInput` and `HasParam` return correct booleans.
- [x] Test `BuildArgs` with both params and inputs present.

## Phase 4: Output Writing (`output.go`, `output_test.go`)

### 4.1 Outputs Collection

- [x] Define `Outputs` struct holding `entries []outputEntry` where `outputEntry` is `{name string, value IntoOutput}`.
- [x] Implement `NewOutputs() *Outputs` constructor.
- [x] Implement `(o *Outputs) Add(name string, value IntoOutput)` — appends a named output.
- [x] Implement `Outputs` satisfying `IntoOutput`: `WriteTo` iterates entries, calls each value's `WriteTo(outputDir/<name>)`.
- [x] Implement `Outputs` satisfying `SpadeType` with sentinel values (`"__outputs__"`) for internal routing (matching Rust pattern).

### 4.2 Block Manifest Reading

- [x] Implement `ReadBlockManifest(base string) map[string]any` — checks (in order):
  1. `SPADE_BLOCK_MANIFEST` environment variable → if set and file exists, read and parse YAML, return the `outputs` key as `map[string]any`.
  2. `<base>/block.yaml` → if exists, read and parse YAML, return the `outputs` key.
  3. Return `nil` if neither found.

### 4.3 Output Writing Logic

- [x] Implement `WriteOutputs(result IntoOutput, base string, manifestOutputs map[string]any) error`:
  1. Get `DefaultOutputName()` from the result.
  2. If `"__outputs__"` (Outputs collection): call `result.WriteTo(<base>/outputs/)` which handles its own subdirectories.
  3. If `"__none__"` (nil/unit result): return nil immediately, write nothing.
  4. Otherwise (single output): determine output name — if manifest has exactly 1 key, use that key; otherwise use `DefaultOutputName()`. Call `result.WriteTo(<base>/outputs/<name>)`.

### 4.4 Unit Output

- [x] Implement a `unitOutput` unexported type (or use a sentinel) for handlers that return no output. `WriteTo` is a no-op. `DefaultOutputName()` returns `"__none__"`.

### 4.5 Tests

- [x] Test writing a single `RasterFile` output: create temp source file, call `WriteOutputs`, verify file copied to `outputs/raster/<filename>`.
- [x] Test writing a single `File` output: verify file copied to `outputs/file/<filename>`.
- [x] Test writing a `RasterFileCollection` output: verify all files copied to `outputs/rasters/`.
- [x] Test writing a `Directory` output: verify recursive copy to `outputs/directory/`.
- [x] Test writing with a manifest that has a single custom output name: verify the custom name is used.
- [x] Test writing multiple named outputs via `Outputs`: verify each output in its own subdirectory under `outputs/`.
- [x] Test writing a unit/nil output: verify `outputs/` directory remains empty.
- [x] Test `ReadBlockManifest` with no manifest (returns nil).
- [x] Test `ReadBlockManifest` with `block.yaml` present.
- [x] Test `ReadBlockManifest` with `SPADE_BLOCK_MANIFEST` env var set (takes precedence over `block.yaml`).
- [x] Test filename preservation (original filename is kept in output).

## Phase 5: Run Function (`run.go`, `run_test.go`)

### 5.1 Core Run Function

- [x] Implement `Run[O IntoOutput](handler func(*Args) (O, error))` as the main entry point:
  1. Calls `BuildArgs(".")`.
  2. Calls `handler(args)`.
  3. Reads block manifest via `ReadBlockManifest(".")`.
  4. Calls `WriteOutputs(result, ".", manifest)`.
  5. On any error, prints `"spade: <error>"` to stderr and calls `os.Exit(1)`.
- [x] Implement `RunAt[O IntoOutput](base string, handler func(*Args) (O, error)) error` — same logic but at a configurable base path, returns error instead of exiting. Used for testing.

### 5.2 Handler Signatures

The generic `O` constraint allows handlers to return:
- Any single type implementing `IntoOutput` (e.g., `RasterFile`, `File`, `Directory`, `FileCollection`)
- `*Outputs` for multiple named outputs
- A no-output sentinel for handlers that produce no output

- [x] For handlers with no output, provide `RunNoOutput(handler func(*Args) error)` which internally wraps to return the unit sentinel and delegates to `RunAt`.

### 5.3 Tests

- [x] Test a simple handler that reads an input and returns no output: verify handler is called, `outputs/` remains empty.
- [x] Test a handler that reads params and inputs: verify correct values are passed through `Args`.
- [x] Test a handler that returns a `RasterFile`: verify file appears in `outputs/raster/`.
- [x] Test a handler that returns `*Outputs` with multiple named outputs: verify each output in its subdirectory.
- [x] Test a handler that returns an error: verify the error propagates from `RunAt`.
- [x] Test a full workflow: params + multiple inputs + processing + single output. Verify all values flow correctly.

## Phase 6: Manifest Builder (`build.go`, `build_test.go`)

### 6.1 ManifestBuilder

- [x] Define `ManifestBuilder` struct with unexported fields: `description string`, `inputs []manifestField`, `outputs []manifestField`. `manifestField` is `{name string, info ManifestInfo}`.
- [x] Implement `NewManifestBuilder() *ManifestBuilder` constructor.
- [x] Implement `(b *ManifestBuilder) Description(desc string) *ManifestBuilder` — sets description, returns builder for chaining.
- [x] Implement `ManifestInput[T SpadeType](b *ManifestBuilder, name string) *ManifestBuilder` as a generic free function (since Go methods can't have type params) — calls `T.ManifestEntry()` to get the `ManifestInfo`, appends to inputs list, returns builder.
- [x] Implement `ManifestOutput[T SpadeType](b *ManifestBuilder, name string) *ManifestBuilder` — same pattern for outputs.
- [x] Implement `(b *ManifestBuilder) Build() map[string]any` — returns a map with:
  - `"description"` key (if set) → string value
  - `"inputs"` key → `map[string]any` where each input name maps to `{"type": ..., "format": ..., "item_type": ...}` (omitting empty fields)
  - `"outputs"` key → same structure as inputs

### 6.2 Scalar Type ManifestInfo

For use with the ManifestBuilder, we need `ManifestInfo` values for scalar types. Since Go doesn't allow implementing interfaces on primitive types, provide named types or manifest-only helper functions:

- [x] `StringType` — a type alias or struct satisfying `SpadeType` with `ManifestEntry()` returning `{TypeName: "string"}`.
- [x] `NumberType` — satisfying `SpadeType` with `ManifestEntry()` returning `{TypeName: "number"}`.
- [x] `BoolType` — satisfying `SpadeType` with `ManifestEntry()` returning `{TypeName: "boolean"}`.

### 6.3 Tests

- [x] Test building a manifest with a simple `File` input.
- [x] Test building with typed file inputs (`RasterFile`, `VectorFile`) — verify `format` is set.
- [x] Test building with scalar inputs (`StringType`, `NumberType`, `BoolType`).
- [x] Test building with collection inputs — verify `item_type` is set.
- [x] Test building with output declarations.
- [x] Test building with a description.
- [x] Test building with no description (key absent from result).
- [x] Test building with mixed inputs (files, scalars, collections) and outputs.
- [x] Test `Directory` and `JsonFile` inputs.

## Phase 7: Module Configuration

- [x] Update `go.mod` to add `gopkg.in/yaml.v3` dependency.
- [x] Run `go mod tidy` to resolve all dependencies.
- [x] Verify all tests pass with `go test ./...`.
- [x] Verify the public API surface is clean: the package exports only the types, interfaces, functions, and errors that block authors need.

## Public API Summary

After implementation, the `spade` package exports:

**Types:** `File`, `RasterFile`, `VectorFile`, `TabularFile`, `JsonFile`, `Directory`, `FileCollection`, `RasterFileCollection`, `VectorFileCollection`, `TabularFileCollection`

**Constructors:** `NewFile`, `NewRasterFile`, `NewVectorFile`, `NewTabularFile`, `NewJsonFile`, `NewDirectory`, `NewFileCollection`, `NewRasterFileCollection`, `NewVectorFileCollection`, `NewTabularFileCollection`

**Interfaces:** `SpadeType`, `FromInput`, `IntoOutput`, `ManifestInfo`

**Args:** `Args`, `Input[T]`, `Param[T]`, `BuildArgs`

**Run:** `Run[O]`, `RunNoOutput`, `RunAt[O]`

**Output:** `Outputs`, `NewOutputs`, `WriteOutputs`, `ReadBlockManifest`

**Build:** `ManifestBuilder`, `NewManifestBuilder`, `ManifestInput[T]`, `ManifestOutput[T]`, `StringType`, `NumberType`, `BoolType`

**Errors:** `ErrInputNotFound`, `ErrParamNotFound`, `ErrEmptyInputDir`, `ErrTypeMismatch`

## Usage Example

```go
package main

import "spade"

func handler(args *spade.Args) (spade.RasterFile, error) {
    source, err := spade.Input[spade.RasterFile](args, "source")
    if err != nil {
        return spade.RasterFile{}, err
    }
    resolution, err := spade.Param[float64](args, "resolution")
    if err != nil {
        return spade.RasterFile{}, err
    }
    // process the raster using source.Path and resolution...
    return spade.NewRasterFile("result.tif"), nil
}

func main() {
    spade.Run(handler)
}
```

## Multiple Outputs Example

```go
package main

import "spade"

func handler(args *spade.Args) (*spade.Outputs, error) {
    source, err := spade.Input[spade.RasterFile](args, "source")
    if err != nil {
        return nil, err
    }
    _ = source
    outputs := spade.NewOutputs()
    outputs.Add("raster", spade.NewRasterFile("result.tif"))
    outputs.Add("stats", spade.NewJsonFile("stats.json"))
    return outputs, nil
}

func main() {
    spade.Run(handler)
}
```
