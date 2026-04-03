# Rust Library Implementation Plan

This plan describes the implementation of the Spade runtime library for Rust, following the patterns established by the Python, TypeScript, and R implementations while adapting to Rust's type system and idioms.

## Design Notes

Rust cannot introspect function signatures at runtime like Python (`get_type_hints`) or R (`formals`). The Rust library therefore uses a different approach:

- **Handler receives an `Args` struct** instead of individual named parameters. The user calls typed accessor methods (`args.input::<RasterFile>("source")`, `args.param::<f64>("resolution")`) to retrieve inputs and parameters.
- **A `FromInput` trait** allows each Spade type to define how it is constructed from raw filesystem data (single file, multiple files, or directory path).
- **An `IntoOutput` trait** allows each Spade type to define how it writes itself to the `outputs/` directory.
- **`Outputs` struct** wraps a `HashMap<String, Box<dyn IntoOutput>>` for handlers that produce multiple named outputs.
- **`build()` uses a `ManifestBuilder`** (fluent API) instead of signature introspection to declare inputs/outputs for manifest generation.

The library is a **library crate** (`lib.rs`), not a binary. Block authors depend on `spade` in their `Cargo.toml` and call `spade::run(handler)` from their `main()`.

---

## Phase 1: Project Setup & Configuration

- [x] Rename the package in `Cargo.toml` from `rust` to `spade`.
- [x] Change `Cargo.toml` to declare both a library (`lib`) and remove the default binary target. Delete `src/main.rs` and create `src/lib.rs`.
- [x] Add dependencies to `Cargo.toml`:
  - `serde` (with `derive` feature) for serialization/deserialization.
  - `serde_yaml` for reading `params.yaml` and `block.yaml`.
  - `serde_json` for JSON support (used by `JsonFile` and potential manifest operations).
  - `thiserror` for structured error types.
- [x] Add dev-dependencies:
  - `tempfile` for creating temporary directories in tests.
- [x] Create the module file structure under `src/`:
  - `lib.rs` (public API re-exports)
  - `types.rs` (type definitions)
  - `error.rs` (error types)
  - `scanning.rs` (load_params, scan_inputs, build_args)
  - `output.rs` (read_block_manifest, write_outputs)
  - `run.rs` (the `run()` function)
  - `build.rs` (ManifestBuilder and `build()`)

---

## Phase 2: Error Handling (`error.rs`)

- [x] Define a `SpadeError` enum using `thiserror::Error` with variants:
  - `IoError(#[from] std::io::Error)` -- wraps filesystem errors.
  - `YamlError(#[from] serde_yaml::Error)` -- wraps YAML parse errors.
  - `JsonError(#[from] serde_json::Error)` -- wraps JSON parse errors.
  - `InputNotFound { name: String }` -- requested input name does not exist in the `inputs/` directory.
  - `ParamNotFound { name: String }` -- requested parameter name does not exist in `params.yaml`.
  - `EmptyInputDir { name: String }` -- an input subdirectory exists but contains no files.
  - `TypeMismatch { name: String, expected: &'static str, found: &'static str }` -- input value cannot be converted to the requested type.
  - `HandlerError(Box<dyn std::error::Error + Send + Sync>)` -- wraps user handler errors.
- [x] Define a `Result<T>` type alias: `pub type Result<T> = std::result::Result<T, SpadeError>`.

---

## Phase 3: Type System (`types.rs`)

### 3.1 Base Structs

- [x] Define `File` struct with a single public field `path: String`. Implement `Clone`, `Debug`, `PartialEq`.
- [x] Define `Directory` struct with a single public field `path: String`. Implement `Clone`, `Debug`, `PartialEq`.
- [x] Define `FileCollection` struct with a single public field `paths: Vec<String>`. Implement `Clone`, `Debug`, `PartialEq`.

### 3.2 Specialized File Types

- [x] Define `RasterFile` struct wrapping `path: String`. Same derives as `File`.
- [x] Define `VectorFile` struct wrapping `path: String`. Same derives.
- [x] Define `TabularFile` struct wrapping `path: String`. Same derives.
- [x] Define `JsonFile` struct wrapping `path: String`. Same derives.

### 3.3 Specialized Collection Types

- [x] Define `RasterFileCollection` struct wrapping `paths: Vec<String>`. Same derives as `FileCollection`.
- [x] Define `VectorFileCollection` struct wrapping `paths: Vec<String>`. Same derives.
- [x] Define `TabularFileCollection` struct wrapping `paths: Vec<String>`. Same derives.

### 3.4 Constructor Methods

- [x] Implement `File::new(path: impl Into<String>) -> Self` and equivalent `new()` for every type listed above. For collection types: `new(paths: Vec<String>) -> Self`.

### 3.5 `SpadeType` Trait

- [x] Define a trait `SpadeType` with:
  - `fn type_name() -> &'static str` -- returns the Spade type string (e.g., `"file"`, `"raster"`, `"directory"`, `"collection"`).
  - `fn default_output_name() -> &'static str` -- returns the default output directory name (e.g., `"file"`, `"raster"`, `"rasters"`).
- [x] Implement `SpadeType` for all 10 types with the following mappings:

  | Struct                    | `type_name()`   | `default_output_name()` |
  |---------------------------|-----------------|------------------------|
  | `File`                    | `"file"`        | `"file"`               |
  | `RasterFile`              | `"file"`        | `"raster"`             |
  | `VectorFile`              | `"file"`        | `"vector"`             |
  | `TabularFile`             | `"file"`        | `"tabular"`            |
  | `JsonFile`                | `"json"`        | `"json"`               |
  | `Directory`               | `"directory"`   | `"directory"`          |
  | `FileCollection`          | `"collection"`  | `"files"`              |
  | `RasterFileCollection`    | `"collection"`  | `"rasters"`            |
  | `VectorFileCollection`    | `"collection"`  | `"vectors"`            |
  | `TabularFileCollection`   | `"collection"`  | `"tables"`             |

### 3.6 `FromInput` Trait

- [x] Define a trait `FromInput: Sized` with:
  - `fn from_single_file(path: String) -> crate::Result<Self>` -- construct from a single file path.
  - `fn from_multiple_files(paths: Vec<String>) -> crate::Result<Self>` -- construct from multiple file paths.
  - `fn from_directory(path: String) -> crate::Result<Self>` -- construct from a directory path.
- [x] Implement `FromInput` for all single-file types (`File`, `RasterFile`, `VectorFile`, `TabularFile`, `JsonFile`):
  - `from_single_file`: return `Self { path }`.
  - `from_multiple_files`: return `Self { path: paths[0] }` (take first file, matching Python behavior).
  - `from_directory`: return `TypeMismatch` error.
- [x] Implement `FromInput` for `Directory`:
  - `from_directory`: return `Self { path }`.
  - `from_single_file` / `from_multiple_files`: return `TypeMismatch` error.
- [x] Implement `FromInput` for all collection types (`FileCollection`, `RasterFileCollection`, `VectorFileCollection`, `TabularFileCollection`):
  - `from_multiple_files`: return `Self { paths }`.
  - `from_single_file`: return `Self { paths: vec![path] }`.
  - `from_directory`: return `TypeMismatch` error.

### 3.7 `IntoOutput` Trait

- [x] Define a trait `IntoOutput` with:
  - `fn write_to(self, output_dir: &Path) -> crate::Result<()>` -- write the value into the given output subdirectory (e.g., `outputs/raster/`).
  - `fn default_output_name(&self) -> &'static str` -- returns the default name for this output type.
- [x] Implement `IntoOutput` for all single-file types: copy the file at `self.path` into `output_dir/` preserving the filename (using `std::fs::copy`).
- [x] Implement `IntoOutput` for `Directory`: copy all contents of `self.path` into `output_dir/` recursively (files via `std::fs::copy`, subdirectories recursively).
- [x] Implement `IntoOutput` for all collection types: copy each file in `self.paths` into `output_dir/` preserving filenames.

---

## Phase 4: Input Scanning (`scanning.rs`)

### 4.1 `load_params`

- [x] Implement `pub fn load_params() -> crate::Result<HashMap<String, serde_yaml::Value>>`:
  - Check if `params.yaml` exists in the current working directory.
  - If missing, return an empty `HashMap`.
  - If present, read and parse with `serde_yaml`. If the file is empty (parses to `Null`), return an empty `HashMap`.
  - Return the top-level mapping as `HashMap<String, serde_yaml::Value>`.

### 4.2 `InputEntry` enum

- [x] Define an internal enum `InputEntry` to represent raw scanned input data:
  ```
  SingleFile(String)       // path to single file
  MultipleFiles(Vec<String>)  // paths to multiple files
  DirectoryInput(String)   // path to the input subdirectory itself
  ```

### 4.3 `scan_inputs`

- [x] Implement `pub fn scan_inputs() -> crate::Result<HashMap<String, InputEntry>>`:
  - Check if `inputs/` directory exists. If not, return empty `HashMap`.
  - Iterate over sorted subdirectories of `inputs/`.
  - For each subdirectory:
    - List all files (non-directories) in sorted order.
    - If no files exist, return `SpadeError::EmptyInputDir`.
    - If exactly one file, store as `InputEntry::SingleFile(path)`.
    - If multiple files, store as `InputEntry::MultipleFiles(paths)`.
  - Return the map of `name -> InputEntry`.

---

## Phase 5: Args Struct (`scanning.rs`)

- [x] Define `pub struct Args` with two fields:
  - `params: HashMap<String, serde_yaml::Value>` -- scalar parameters from `params.yaml`.
  - `inputs: HashMap<String, InputEntry>` -- file/directory inputs from `inputs/`.

- [x] Implement `Args::input<T: FromInput>(&self, name: &str) -> crate::Result<T>`:
  - Look up `name` in `self.inputs`.
  - If not found, return `SpadeError::InputNotFound`.
  - Dispatch to the appropriate `FromInput` method based on the `InputEntry` variant.

- [x] Implement `Args::param<T: serde::de::DeserializeOwned>(&self, name: &str) -> crate::Result<T>`:
  - Look up `name` in `self.params`.
  - If not found, return `SpadeError::ParamNotFound`.
  - Deserialize the `serde_yaml::Value` into `T` using `serde_yaml::from_value`.

- [x] Implement `Args::has_input(&self, name: &str) -> bool` for checking input existence.
- [x] Implement `Args::has_param(&self, name: &str) -> bool` for checking param existence.

- [x] Implement `pub fn build_args() -> crate::Result<Args>`:
  - Call `load_params()` to get params.
  - Call `scan_inputs()` to get inputs.
  - Return `Args { params, inputs }`.

---

## Phase 6: Output Handling (`output.rs`)

### 6.1 `Outputs` Struct

- [x] Define `pub struct Outputs` wrapping an internal `Vec<(String, Box<dyn IntoOutput>)>` (ordered list of named outputs).
- [x] Implement `Outputs::new() -> Self` (empty).
- [x] Implement `Outputs::add<T: IntoOutput + 'static>(&mut self, name: impl Into<String>, value: T)` -- add a named output.
- [x] Implement `Outputs::single<T: IntoOutput + SpadeType + 'static>(value: T) -> Self` -- convenience for single-output handlers; uses `T::default_output_name()` as the name.
- [x] Implement `IntoOutput` for `Outputs` itself -- iterates entries and writes each to `outputs/<name>/`.

### 6.2 `read_block_manifest`

- [x] Implement `pub fn read_block_manifest() -> Option<HashMap<String, serde_yaml::Value>>`:
  - Check `SPADE_BLOCK_MANIFEST` environment variable first. If set and the file exists, read and return the `outputs` key.
  - Otherwise, check `block.yaml` in the current working directory. If it exists, read and return the `outputs` key.
  - If neither exists, return `None`.

### 6.3 `write_outputs`

- [x] Implement `pub fn write_outputs<T: IntoOutput>(result: T, manifest_outputs: Option<&HashMap<String, serde_yaml::Value>>) -> crate::Result<()>`:
  - Create the `outputs/` directory if it doesn't exist.
  - If `result` is `Outputs` (multiple named outputs), iterate and write each to `outputs/<name>/`.
  - If `result` is a single value and the manifest has exactly one output declaration, use that output's name.
  - Otherwise, use `result.default_output_name()` to determine the directory name.
  - Call `result.write_to(output_dir)` to perform the actual file copy.

### 6.4 Helper: `copy_dir_recursive`

- [x] Implement a private helper `fn copy_dir_recursive(src: &Path, dst: &Path) -> io::Result<()>` that recursively copies directory contents, matching the Python `shutil.copytree` behavior.

---

## Phase 7: Run Function (`run.rs`)

- [x] Implement `pub fn run<F, O>(handler: F)` where `F: FnOnce(Args) -> std::result::Result<O, Box<dyn std::error::Error + Send + Sync>>` and `O: IntoOutput + SpadeType`:
  1. Call `build_args()` to construct the `Args` from the filesystem.
  2. Call `handler(args)` with the constructed arguments.
  3. If the handler returns an error, print the error to stderr and exit with code 1 (matching the spec: non-zero exit code signals failure).
  4. If the handler returns `Ok(result)`, call `read_block_manifest()` and then `write_outputs(result, manifest)`.
  5. If output writing fails, print the error to stderr and exit with code 1.

- [x] Also provide an overload or separate function `pub fn run_with_outputs<F>(handler: F)` where the handler returns `Result<Outputs, ...>` for multi-output blocks. Alternatively, implement `SpadeType` for `Outputs` with a sentinel `default_output_name()` so the same `run()` works for both cases.

- [x] Ensure that the `run()` function signature is ergonomic for the common case. The simplest invocation should look like:
  ```rust
  use spade::{run, Args, RasterFile};

  fn handler(args: Args) -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
      let source: RasterFile = args.input("source")?;
      Ok(RasterFile::new("result.tif"))
  }

  fn main() {
      run(handler);
  }
  ```

---

## Phase 8: Build Function (`build.rs`)

- [x] Define a `ManifestEntry` struct (or use `HashMap<String, String>`) to represent a single input/output declaration in the manifest (fields: `type`, optional `format`, optional `item_type`).

- [x] Define a `ManifestBuilder` struct with fields:
  - `description: Option<String>`
  - `inputs: Vec<(String, ManifestEntry)>`
  - `outputs: Vec<(String, ManifestEntry)>`

- [x] Implement `ManifestBuilder::new() -> Self`.
- [x] Implement `ManifestBuilder::description(mut self, desc: impl Into<String>) -> Self`.
- [x] Implement `ManifestBuilder::input<T: SpadeType>(mut self, name: impl Into<String>) -> Self`:
  - Use a `ManifestInfo` trait (or extend `SpadeType`) to provide manifest metadata.
  - Map types to manifest entries using the following table:

  | Rust Type                 | Manifest                                                  |
  |---------------------------|----------------------------------------------------------|
  | `File`                    | `{ type: "file" }`                                       |
  | `RasterFile`              | `{ type: "file", format: "GeoTIFF" }`                    |
  | `VectorFile`              | `{ type: "file", format: "GeoJSON" }`                    |
  | `TabularFile`             | `{ type: "file", format: "CSV" }`                        |
  | `JsonFile`                | `{ type: "json" }`                                       |
  | `Directory`               | `{ type: "directory" }`                                  |
  | `FileCollection`          | `{ type: "collection", item_type: "file" }`              |
  | `RasterFileCollection`    | `{ type: "collection", item_type: "file", format: "GeoTIFF" }` |
  | `VectorFileCollection`    | `{ type: "collection", item_type: "file", format: "GeoJSON" }` |
  | `TabularFileCollection`   | `{ type: "collection", item_type: "file", format: "CSV" }` |
  | `String`                  | `{ type: "string" }`                                     |
  | `f64` / `i64`             | `{ type: "number" }`                                     |
  | `bool`                    | `{ type: "boolean" }`                                    |

- [x] Implement `ManifestBuilder::output<T: SpadeType>(mut self, name: impl Into<String>) -> Self`.
- [x] Implement `ManifestBuilder::build(self) -> HashMap<String, serde_yaml::Value>` that produces the final manifest dict.
- [x] Add scalar type support: implement `SpadeType` (or a separate `ManifestInfo` trait) for `String`, `f64`, `i64`, `i32`, `f32`, `bool` so they can be used with `.input::<String>("name")`.

- [x] Implement a top-level convenience function `pub fn build() -> ManifestBuilder` that returns a new builder.

---

## Phase 9: Public API (`lib.rs`)

- [x] Re-export all public types from `types.rs`: `File`, `Directory`, `RasterFile`, `VectorFile`, `TabularFile`, `JsonFile`, `FileCollection`, `RasterFileCollection`, `VectorFileCollection`, `TabularFileCollection`.
- [x] Re-export traits: `SpadeType`, `FromInput`, `IntoOutput`.
- [x] Re-export `Args` from `scanning.rs`.
- [x] Re-export `Outputs` from `output.rs`.
- [x] Re-export `run` from `run.rs`.
- [x] Re-export `build` and `ManifestBuilder` from `build.rs`.
- [x] Re-export `SpadeError` and `Result` from `error.rs`.

---

## Phase 10: Tests

All tests use `tempfile::TempDir` to create isolated working directories and `std::env::set_current_dir` to simulate the block execution environment. Each test restores the original working directory on completion.

### 10.1 Type Tests (`tests/types.rs` or inline `#[cfg(test)]` in `types.rs`)

- [x] Test construction of each type via `new()`.
- [x] Test that `File::new("/tmp/data.tif").path == "/tmp/data.tif"`.
- [x] Test that `Directory::new("/tmp/source").path == "/tmp/source"`.
- [x] Test that `FileCollection::new(vec![...]).paths` matches input.
- [x] Test that `RasterFile`, `VectorFile`, `TabularFile`, `JsonFile` all construct correctly.
- [x] Test that `RasterFileCollection`, `VectorFileCollection`, `TabularFileCollection` all construct correctly.
- [x] Test that `Clone` produces equal values.
- [x] Test `SpadeType::type_name()` returns correct string for each type.
- [x] Test `SpadeType::default_output_name()` returns correct string for each type.

### 10.2 Scanning Tests (`tests/scanning.rs` or inline in `scanning.rs`)

- [x] **`load_params` tests:**
  - Test loading basic params (`buffer_distance: 30, method: bilinear`).
  - Test empty `params.yaml` returns empty map.
  - Test missing `params.yaml` returns empty map.

- [x] **`scan_inputs` tests:**
  - Test single file input produces `SingleFile` entry.
  - Test multiple files produce `MultipleFiles` entry.
  - Test empty input subdirectory returns `EmptyInputDir` error.
  - Test missing `inputs/` directory returns empty map.
  - Test multiple input subdirectories are all scanned.

- [x] **`Args` tests:**
  - Test `args.input::<RasterFile>("source")` returns correct `RasterFile` for single-file input.
  - Test `args.input::<RasterFileCollection>("tiles")` returns correct collection for multi-file input.
  - Test `args.input::<Directory>("source")` returns correct `Directory` when input is a directory.
  - Test `args.param::<f64>("resolution")` deserializes correctly.
  - Test `args.param::<String>("method")` deserializes correctly.
  - Test `args.param::<bool>("normalize")` deserializes correctly.
  - Test `args.input("missing")` returns `InputNotFound` error.
  - Test `args.param("missing")` returns `ParamNotFound` error.

- [x] **`build_args` tests:**
  - Test that params and inputs are both loaded and accessible.

### 10.3 Output Tests (`tests/output.rs` or inline in `output.rs`)

- [x] Test `write_outputs` with a single `RasterFile` -- file is copied to `outputs/raster/<filename>`.
- [x] Test `write_outputs` with a single `File` -- file is copied to `outputs/file/<filename>`.
- [x] Test `write_outputs` with a `FileCollection` -- all files copied to `outputs/files/`.
- [x] Test `write_outputs` with a `Directory` -- directory contents copied recursively to `outputs/directory/`.
- [x] Test `write_outputs` with manifest specifying a custom output name -- uses manifest name instead of inferred name.
- [x] Test `write_outputs` with `Outputs` containing multiple named outputs -- each written to its own subdirectory.
- [x] Test that original filenames are preserved in the output directory.
- [x] Test `read_block_manifest` returns `None` when no manifest exists.
- [x] Test `read_block_manifest` reads from `block.yaml` in cwd.
- [x] Test `read_block_manifest` reads from `SPADE_BLOCK_MANIFEST` env var path.
- [x] Test `read_block_manifest` prefers env var over `block.yaml`.

### 10.4 Run Tests (`tests/run.rs` or inline in `run.rs`)

- [x] Test simple handler with one file input -- handler receives correct `Args`, no output.
- [x] Test handler with params and file inputs -- handler receives both.
- [x] Test handler returning a single `RasterFile` -- output is written to `outputs/`.
- [x] Test handler returning `Outputs` with multiple entries -- all outputs written.
- [x] Test handler error propagates (non-zero exit).
- [x] Test handler with no return value (returns unit-like) -- no output files created.
- [x] Test full end-to-end workflow: params + inputs -> handler -> outputs.

### 10.5 Build Tests (`tests/build.rs` or inline in `build.rs`)

- [x] Test `ManifestBuilder` with a simple file input.
- [x] Test typed file inputs produce correct format fields (`RasterFile` -> `GeoTIFF`).
- [x] Test scalar inputs (`String`, `f64`, `bool`) produce correct type fields.
- [x] Test collection inputs produce correct `type: collection` with `item_type`.
- [x] Test output declarations produce correct manifest entries.
- [x] Test description field is included when set.
- [x] Test building manifest with mixed file and scalar inputs.

---

## Phase 11: Documentation & Examples

- [x] Add doc comments (`///`) to all public types, traits, and functions.
- [x] Add a module-level doc comment in `lib.rs` showing the minimal usage example:
  ```rust
  use spade::{run, Args, RasterFile};

  fn handler(args: Args) -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
      let source: RasterFile = args.input("source")?;
      let resolution: f64 = args.param("resolution")?;
      // process the raster...
      Ok(RasterFile::new("result.tif"))
  }

  fn main() {
      run(handler);
  }
  ```
- [x] Add a multi-output example in the `Outputs` doc comment:
  ```rust
  use spade::{run, Args, Outputs, RasterFile, JsonFile};

  fn handler(args: Args) -> Result<Outputs, Box<dyn std::error::Error + Send + Sync>> {
      let source: RasterFile = args.input("source")?;
      let mut outputs = Outputs::new();
      outputs.add("raster", RasterFile::new("result.tif"));
      outputs.add("stats", JsonFile::new("stats.json"));
      Ok(outputs)
  }

  fn main() {
      run(handler);
  }
  ```

---

## Verification Checklist

After implementation, verify the following:

- [x] `cargo build` compiles without errors or warnings.
- [x] `cargo test` passes all tests.
- [x] `cargo clippy` reports no warnings.
- [x] `cargo doc` generates documentation without warnings.
- [x] The public API exports match the Python library's `__all__` list (adapted for Rust naming): `File`, `Directory`, `RasterFile`, `VectorFile`, `TabularFile`, `JsonFile`, `FileCollection`, `RasterFileCollection`, `VectorFileCollection`, `TabularFileCollection`, `run`, `build`.
- [x] The library can be used as a dependency by a Rust block collection (the user adds `spade = { path = "..." }` to their `Cargo.toml`).
- [x] All test scenarios from the Python test suite have equivalent coverage in Rust.
