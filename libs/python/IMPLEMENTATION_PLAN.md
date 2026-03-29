# Python Library Implementation Plan

This document describes the implementation plan for the Spade Python library (`spade`). The library enables Python block authors to define handler functions and run them within the Spade execution environment.

---

## Current State

The library has basic stubs with several issues:

- **Package name**: Currently `python`, must be renamed to `spade` (spec imports: `from spade import run, File`)
- **`src/types.py`**: Has `File` and `Directory` base models only; missing all specialized subtypes and collections
- **`src/run.py`**: Has skeleton `run()` and `scan_directory_for_inputs()`; multiple bugs:
  - Uses `json.load()` instead of `yaml.safe_load()` for `params.yaml`
  - References `params.yml` instead of `params.yaml`
  - Uses `+` for dict merge instead of `|`
  - No input scanning logic
  - No output handling
  - No type hint inspection
- **`src/build.py`**: Empty `build()` stub
- **`src/python/__init__.py`**: Placeholder `hello()` function
- **Dependencies**: Missing `pyyaml` in `pyproject.toml`
- **Tests**: None exist

---

## 1. Project Structure Reorganization

### 1.1 Rename package from `python` to `spade`

**Files to change:**
- Rename `src/python/` directory to `src/spade/`
- Update `pyproject.toml`: change `name = "python"` to `name = "spade"`
- Update `src/spade/__init__.py` to export public API

**Target directory structure:**
```
libs/python/
  pyproject.toml
  src/
    spade/
      __init__.py      # Public API exports
      py.typed          # PEP 561 marker (already exists as src/python/py.typed)
      types.py          # Type definitions (File, Directory, subtypes, collections)
      run.py            # run() function and input/output handling
      build.py          # build() function for manifest generation
      _scanning.py      # Internal: working directory scanning logic
      _output.py        # Internal: output writing logic
  tests/
    __init__.py
    conftest.py         # Shared fixtures (working directory setup, etc.)
    test_types.py       # Tests for type classes
    test_scanning.py    # Tests for input scanning
    test_output.py      # Tests for output writing
    test_run.py         # Integration tests for full run() flow
    test_build.py       # Tests for build() function
```

### 1.2 Update `pyproject.toml`

```toml
[project]
name = "spade"
version = "0.1.0"
description = "Spade block authoring library for Python"
readme = "README.md"
authors = [
    { name = "krbundy", email = "kenneth.bundy@maine.edu" }
]
requires-python = ">=3.12"
dependencies = [
    "pydantic>=2.12.5",
    "pyyaml>=6.0",
]

[project.optional-dependencies]
dev = [
    "pytest>=9.0.2",
]

[build-system]
requires = ["uv_build>=0.10.0,<0.11.0"]
build-backend = "uv_build"
```

**Note**: Move `pytest` from `dependencies` to `[project.optional-dependencies] dev`. It is a dev-only dependency. The `uv run pytest` command will still work because `uv` resolves optional deps when running tools.

---

## 2. Type Definitions (`src/spade/types.py`)

### 2.1 Base Types

Two base models represent single-file and single-directory inputs/outputs:

```python
from pydantic import BaseModel

class File(BaseModel):
    """Base class for single-file inputs/outputs."""
    path: str

class Directory(BaseModel):
    """Base class for directory-based inputs/outputs."""
    path: str
```

### 2.2 File Subtypes

Each subtype inherits from `File` and represents a specific data format. These are thin wrappers providing type safety and clarity in function signatures:

```python
class RasterFile(File):
    """Raster data file (e.g., GeoTIFF)."""
    pass

class VectorFile(File):
    """Vector data file (e.g., GeoJSON, Shapefile)."""
    pass

class TabularFile(File):
    """Tabular data file (e.g., CSV, Parquet)."""
    pass

class JsonFile(File):
    """JSON data file."""
    pass
```

### 2.3 Collection Types

Collections represent variable-length sets of files. They have a `paths` field (list of strings) instead of a single `path`:

```python
class FileCollection(BaseModel):
    """Base class for a collection of files."""
    paths: list[str]

class RasterFileCollection(FileCollection):
    """Collection of raster data files."""
    pass

class VectorFileCollection(FileCollection):
    """Collection of vector data files."""
    pass

class TabularFileCollection(FileCollection):
    """Collection of tabular data files."""
    pass
```

### 2.4 Type Registry

Maintain an internal mapping from type classes to their characteristics (is_file, is_directory, is_collection) to support input scanning and output writing:

```python
_FILE_TYPES = (File, RasterFile, VectorFile, TabularFile, JsonFile)
_DIRECTORY_TYPES = (Directory,)
_COLLECTION_TYPES = (FileCollection, RasterFileCollection, VectorFileCollection, TabularFileCollection)
```

This allows `run()` and the scanning/output modules to check `isinstance()` or `issubclass()` to determine how to handle values.

---

## 3. Input Scanning (`src/spade/_scanning.py`)

This module scans the working directory to discover inputs and parameters, then builds the arguments dict for the handler function.

### 3.1 Parameter Loading

Read `params.yaml` from the current working directory using `yaml.safe_load()`:

```python
def load_params() -> dict:
    params_path = Path("params.yaml")
    if not params_path.exists():
        return {}
    with open(params_path, "r") as f:
        params = yaml.safe_load(f)
    return params if params is not None else {}
```

**Key points:**
- Return empty dict if file doesn't exist (some blocks have no params)
- Handle `None` from `yaml.safe_load()` on empty files
- File is always `params.yaml` (not `params.yml`)

### 3.2 Input Directory Scanning

Scan `./inputs/` for subdirectories. Each subdirectory name corresponds to a handler function parameter name. The contents determine the type:

```python
def scan_inputs(type_hints: dict[str, type]) -> dict[str, Any]:
    """Scan the inputs/ directory and build typed arguments.

    Args:
        type_hints: Mapping of parameter name -> expected type from the handler's annotations.

    Returns:
        Dict mapping parameter name -> typed value (File, Directory, Collection, etc.)
    """
```

**Algorithm:**

1. List subdirectories in `./inputs/`
2. For each subdirectory `inputs/<name>/`:
   a. Look up the expected type from `type_hints[name]`
   b. If expected type is a `File` subclass:
      - List files in the subdirectory
      - There should be exactly one file (per spec: "there should only be one file in that directory")
      - Create an instance: `TypeClass(path=str(file_path))`
   c. If expected type is a `Directory` subclass:
      - Use the subdirectory path itself: `Directory(path=str(subdir_path))`
   d. If expected type is a `FileCollection` subclass:
      - List all files in the subdirectory
      - Sort for deterministic ordering
      - Create: `TypeClass(paths=[str(p) for p in sorted(files)])`
   e. If the parameter name is not in `type_hints`, or the type is not a recognized Spade type:
      - Fall back to `File(path=str(first_file))` for a single file
      - Fall back to `FileCollection(paths=...)` for multiple files
3. Return the arguments dict

**Edge cases to handle:**
- Empty subdirectory: raise an error with a clear message
- No type hint for a parameter: use default File/FileCollection based on file count
- Type hint is not a Spade type (e.g., plain `str`): skip, this parameter comes from params.yaml

### 3.3 Combining Parameters and Inputs

```python
def build_function_args(fn: Callable) -> dict[str, Any]:
    """Build the full arguments dict for the handler function."""
    type_hints = get_type_hints(fn)
    params = load_params()
    inputs = scan_inputs(type_hints)

    # Merge: inputs take precedence (they are the file-based arguments)
    # params provides scalar values (strings, numbers, booleans)
    return params | inputs
```

The dict merge `params | inputs` ensures that if there's a name collision (unlikely), the file input takes precedence over a scalar parameter.

---

## 4. Output Handling (`src/spade/_output.py`)

After the handler function returns, `run()` must write the return value(s) to the `outputs/` directory.

### 4.1 Output Name Resolution

The output names come from `block.yaml`. The library needs to know what output names to use. Two strategies, tried in order:

1. **Read `block.yaml`**: The library looks for a `block.yaml` file. The path can be provided via an environment variable `SPADE_BLOCK_MANIFEST` set by the worker, or discovered relative to the entry point script.

2. **Fallback for single outputs**: If there is exactly one output and the handler returns a single value (not a dict), use the output name from the manifest. If no manifest is available, use a default name derived from the return type (e.g., `RasterFile` -> `"raster"`).

3. **Dict return for multiple outputs**: If the handler returns a `dict`, the keys are the output names and the values are the typed outputs. This is validated against the manifest if available.

### 4.2 Writing Outputs

```python
def write_outputs(result: Any, manifest_outputs: dict | None = None) -> None:
    """Write handler return value(s) to the outputs/ directory."""
```

**Algorithm:**

1. Create `outputs/` directory if it doesn't exist
2. Determine the output mapping:
   - If `result` is a `dict`: use keys as output names, values as output values
   - If `result` is a single `File`/`Directory`/`Collection`: wrap in dict with inferred name
   - If `result` is `None`: nothing to write (block has no declared outputs, or produces side effects only)
3. For each `(name, value)` pair:
   a. Create `outputs/<name>/` subdirectory
   b. If value is a `File` subclass:
      - Copy the file at `value.path` into `outputs/<name>/`
      - Preserve the original filename
   c. If value is a `Directory` subclass:
      - Copy the directory contents into `outputs/<name>/`
   d. If value is a `FileCollection` subclass:
      - Copy each file in `value.paths` into `outputs/<name>/`
      - Preserve original filenames

**File operations**: Use `shutil.copy2()` for files (preserves metadata) and `shutil.copytree()` for directories.

### 4.3 Output Name Inference

When no manifest is available and the handler returns a single value, infer the output name from the return type:

```python
_TYPE_TO_DEFAULT_NAME = {
    File: "file",
    RasterFile: "raster",
    VectorFile: "vector",
    TabularFile: "tabular",
    JsonFile: "json",
    Directory: "directory",
    FileCollection: "files",
    RasterFileCollection: "rasters",
    VectorFileCollection: "vectors",
    TabularFileCollection: "tables",
}
```

### 4.4 Manifest Reading

```python
def read_block_manifest() -> dict | None:
    """Attempt to read the block manifest for output declarations.

    Checks (in order):
    1. SPADE_BLOCK_MANIFEST environment variable
    2. block.yaml in the current working directory (placed by worker)

    Returns None if no manifest is found.
    """
```

Read only the `outputs` section from the YAML manifest. This provides output names and types for validation.

---

## 5. The `run()` Function (`src/spade/run.py`)

This is the main entry point for block execution. It ties together scanning, handler invocation, and output writing.

### 5.1 Implementation

```python
def run(fn: Callable) -> None:
    """Execute a handler function as a Spade block.

    1. Load scalar parameters from params.yaml
    2. Scan inputs/ directory for file-based arguments
    3. Build the function arguments dict
    4. Call the handler function
    5. Write return value(s) to outputs/ directory

    Args:
        fn: The handler function to execute. Its type hints determine
            how inputs are loaded and outputs are written.
    """
```

**Detailed flow:**

1. **Load parameters**: Call `load_params()` to read `params.yaml`
2. **Get type hints**: Use `typing.get_type_hints(fn)` to inspect the handler's parameter types and return type
3. **Scan inputs**: Call `scan_inputs(type_hints)` to discover file-based inputs from `./inputs/`
4. **Merge arguments**: Combine params and inputs: `args = params | inputs`
5. **Filter arguments**: Only pass arguments that match the handler's parameter names (from `inspect.signature(fn)`). This prevents unexpected keyword arguments if params.yaml has extra keys.
6. **Call handler**: `result = fn(**filtered_args)`
7. **Read manifest**: Attempt to read block.yaml for output declarations
8. **Write outputs**: Call `write_outputs(result, manifest_outputs)` to write the return value to `outputs/`
9. **Exit**: The block process should exit cleanly (exit code 0) on success. If the handler raises an exception, let it propagate (non-zero exit code signals failure to the worker).

### 5.2 Error Handling

- **Missing required input**: If the handler expects a parameter that is not in params.yaml and not in inputs/, raise a clear error: `"Missing required input: '<name>'. Expected a subdirectory at inputs/<name>/ or a parameter in params.yaml."`
- **Type mismatch**: If a type hint specifies `RasterFile` but the input scanning can't match, raise an error with context.
- **Handler exception**: Let exceptions propagate naturally. The worker captures stderr and the non-zero exit code signals failure.
- **Output write failure**: Raise with context about which output failed and why.

---

## 6. The `build()` Function (`src/spade/build.py`)

The `build()` function is a development-time utility used by the CLI (`spade add`) to generate or update block manifests from a handler function's signature.

### 6.1 Implementation

```python
def build(fn: Callable) -> dict:
    """Generate a block manifest dict from a handler function's signature.

    Inspects the function's type hints and docstring to produce a dict
    that can be serialized to block.yaml.

    Args:
        fn: The handler function to inspect.

    Returns:
        A dict representing the block manifest (inputs, outputs, description).
    """
```

**Algorithm:**

1. **Inspect parameters**: Use `inspect.signature(fn)` and `typing.get_type_hints(fn)` to get parameter names and types
2. **Classify inputs**: For each parameter:
   - If type is a `File` subclass -> input type `file`, with format hint from the subclass
   - If type is a `Directory` subclass -> input type `directory`
   - If type is a `FileCollection` subclass -> input type `collection` with `item_type: file`
   - If type is `str` -> input type `string`
   - If type is `int` or `float` -> input type `number`
   - If type is `bool` -> input type `boolean`
3. **Classify outputs**: Inspect the return type annotation:
   - Single `File` subclass -> one output of type `file`
   - Single `Directory` subclass -> one output of type `directory`
   - `dict` return -> each key/value pair becomes a named output
   - `None` -> no outputs
4. **Extract description**: Use `fn.__doc__` as the block description
5. **Build manifest dict**: Assemble the `inputs` and `outputs` sections
6. **Write files**:
   - Write the manifest dict as YAML to stdout or a specified path
   - Write the docstring to `description.md` if present

### 6.2 Type-to-Manifest Mapping

```python
_PYTHON_TYPE_TO_MANIFEST = {
    File: {"type": "file"},
    RasterFile: {"type": "file", "format": "GeoTIFF"},
    VectorFile: {"type": "file", "format": "GeoJSON"},
    TabularFile: {"type": "file", "format": "CSV"},
    JsonFile: {"type": "json"},
    Directory: {"type": "directory"},
    FileCollection: {"type": "collection", "item_type": "file"},
    RasterFileCollection: {"type": "collection", "item_type": "file", "format": "GeoTIFF"},
    VectorFileCollection: {"type": "collection", "item_type": "file", "format": "GeoJSON"},
    TabularFileCollection: {"type": "collection", "item_type": "file", "format": "CSV"},
    str: {"type": "string"},
    int: {"type": "number"},
    float: {"type": "number"},
    bool: {"type": "boolean"},
}
```

---

## 7. Public API (`src/spade/__init__.py`)

Export all public types and functions from the package root:

```python
from spade.types import (
    File,
    Directory,
    RasterFile,
    VectorFile,
    TabularFile,
    JsonFile,
    FileCollection,
    RasterFileCollection,
    VectorFileCollection,
    TabularFileCollection,
)
from spade.run import run
from spade.build import build

__all__ = [
    "File",
    "Directory",
    "RasterFile",
    "VectorFile",
    "TabularFile",
    "JsonFile",
    "FileCollection",
    "RasterFileCollection",
    "VectorFileCollection",
    "TabularFileCollection",
    "run",
    "build",
]
```

This enables the spec's import style: `from spade import run, File, RasterFile`.

---

## 8. Testing Plan

All tests use `pytest` and run with `uv run pytest`. Tests use `tmp_path` fixtures to create isolated working directories.

### 8.1 `tests/conftest.py` - Shared Fixtures

```python
@pytest.fixture
def work_dir(tmp_path):
    """Create a mock Spade working directory with inputs/, outputs/, logs/."""
    (tmp_path / "inputs").mkdir()
    (tmp_path / "outputs").mkdir()
    (tmp_path / "logs").mkdir()
    original = os.getcwd()
    os.chdir(tmp_path)
    yield tmp_path
    os.chdir(original)
```

Additional fixtures:
- `params_file(work_dir)`: Helper to write a `params.yaml` with given content
- `input_file(work_dir)`: Helper to create an input subdirectory with a file
- `input_collection(work_dir)`: Helper to create an input subdirectory with multiple files

### 8.2 `tests/test_types.py` - Type Tests

- Test `File` creation with valid path
- Test `Directory` creation with valid path
- Test each subtype (RasterFile, VectorFile, etc.) inherits from File
- Test `FileCollection` with list of paths
- Test each collection subtype inherits from FileCollection
- Test Pydantic validation (path must be a string)
- Test serialization/deserialization (model_dump, model_validate)

### 8.3 `tests/test_scanning.py` - Input Scanning Tests

- **`test_load_params_basic`**: Write a `params.yaml` with scalar values, verify they are loaded correctly
- **`test_load_params_empty_file`**: Empty `params.yaml` returns empty dict
- **`test_load_params_missing_file`**: No `params.yaml` returns empty dict
- **`test_scan_single_file_input`**: Create `inputs/raster/data.tif`, verify `File(path="inputs/raster/data.tif")` is returned
- **`test_scan_typed_file_input`**: With type hint `RasterFile`, verify `RasterFile(path=...)` is created
- **`test_scan_directory_input`**: With type hint `Directory`, verify `Directory(path="inputs/source")` is created
- **`test_scan_collection_input`**: With type hint `FileCollection`, verify `FileCollection(paths=[...])` is created with sorted paths
- **`test_scan_multiple_inputs`**: Multiple input subdirectories are all discovered
- **`test_scan_empty_input_dir`**: Empty subdirectory raises an error
- **`test_build_function_args`**: Full integration: params + inputs merged correctly, inputs take precedence
- **`test_untyped_parameter_defaults_to_file`**: Parameter without type hint defaults to `File`

### 8.4 `tests/test_output.py` - Output Writing Tests

- **`test_write_single_file_output`**: Return `File(path="tmp/result.tif")`, verify copied to `outputs/<name>/result.tif`
- **`test_write_raster_file_output`**: Return `RasterFile`, verify correct output
- **`test_write_directory_output`**: Return `Directory(path="tmp/results/")`, verify directory copied
- **`test_write_collection_output`**: Return `FileCollection(paths=[...])`, verify all files copied
- **`test_write_dict_output`**: Return `{"raster": RasterFile(...), "summary": JsonFile(...)}`, verify both outputs written
- **`test_write_none_output`**: Return `None`, verify no files written and no error
- **`test_output_name_inference`**: Verify type-to-name mapping works for each type
- **`test_write_output_preserves_filename`**: Verify the original filename is preserved in the output subdirectory

### 8.5 `tests/test_run.py` - Integration Tests

- **`test_run_simple_handler`**: Handler with one File input and no return, verify it is called with correct args
- **`test_run_with_params_and_inputs`**: Handler with both scalar params and file inputs
- **`test_run_with_typed_inputs`**: Handler with `RasterFile` type hints
- **`test_run_with_output`**: Handler returns a `File`, verify it is written to outputs/
- **`test_run_with_dict_output`**: Handler returns dict of outputs
- **`test_run_handler_exception`**: Handler raises exception, verify it propagates (non-zero exit)
- **`test_run_missing_input`**: Required parameter has no input or param, verify clear error
- **`test_run_full_workflow`**: End-to-end test simulating a real block execution:
  1. Set up working directory with params.yaml and inputs/
  2. Define handler with typed inputs and a return value
  3. Call `run(handler)`
  4. Verify outputs/ contains the correct files

### 8.6 `tests/test_build.py` - Build Function Tests

- **`test_build_simple_function`**: Function with `File` input, verify manifest has correct input declaration
- **`test_build_typed_inputs`**: Function with `RasterFile`, `VectorFile` inputs, verify format hints
- **`test_build_scalar_inputs`**: Function with `str`, `int`, `bool` params, verify manifest types
- **`test_build_with_return_type`**: Function with return type annotation, verify output declaration
- **`test_build_with_docstring`**: Function with docstring, verify description is extracted
- **`test_build_no_type_hints`**: Function without type hints, verify graceful handling
- **`test_build_collection_input`**: Function with `FileCollection` input, verify collection type in manifest

---

## 9. Implementation Order

The implementation should proceed in this order, with each step building on the previous:

### Step 1: Project restructuring [DONE]
1. Rename `src/python/` to `src/spade/`
2. Update `pyproject.toml` (name, dependencies, optional-dependencies)
3. Delete `src/__init__.py` (not needed; `src/spade/__init__.py` is the package root)
4. Verify `uv sync` works with the new structure

### Step 2: Types (`src/spade/types.py`) [DONE]
1. Implement all base types and subtypes
2. Implement collection types
3. Write and run `tests/test_types.py`

### Step 3: Input scanning (`src/spade/_scanning.py`) [DONE]
1. Implement `load_params()`
2. Implement `scan_inputs()`
3. Implement `build_function_args()`
4. Write and run `tests/test_scanning.py`

### Step 4: Output handling (`src/spade/_output.py`) [DONE]
1. Implement `write_outputs()`
2. Implement output name inference
3. Implement optional manifest reading
4. Write and run `tests/test_output.py`

### Step 5: `run()` function (`src/spade/run.py`) [DONE]
1. Implement the complete `run()` function tying everything together
2. Write and run `tests/test_run.py`

### Step 6: `build()` function (`src/spade/build.py`) [DONE]
1. Implement signature inspection and manifest generation
2. Write and run `tests/test_build.py`

### Step 7: Public API and final integration [DONE]
1. Update `src/spade/__init__.py` with all exports
2. Run full test suite: `uv run pytest` -- 65 tests passed
3. Verify the "Hello World" example from the spec works:
   ```python
   from spade import run, File
   def handler(file: File):
       print("Hello World")
   if __name__ == "__main__":
       run(handler)
   ```

---

## 10. Design Decisions and Rationale

### 10.1 Why separate `_scanning.py` and `_output.py`?

These are internal modules (prefixed with `_`) that isolate the input scanning and output writing logic from the main `run()` function. This makes each concern independently testable without having to mock the entire run flow.

### 10.2 Why support both manifest-based and inferred output names?

The spec says output names come from `block.yaml`, but during local development (especially when using `run()` in a script before the manifest exists), inferring from the return type provides a usable default. The manifest-based approach is the primary path; inference is a fallback.

### 10.3 Why use `shutil.copy2()` instead of `shutil.move()`?

Copy preserves the original file, which is safer. The handler may return a file path that the handler function still references. Moving could cause subtle issues. The worker manages cleanup of the working directory after execution.

### 10.4 Why filter function arguments?

The `params.yaml` may contain keys that don't correspond to handler parameters (e.g., metadata added by the worker). Filtering to only the handler's declared parameters prevents `TypeError` from unexpected keyword arguments while still allowing block authors to use `**kwargs` if they want all parameters.

### 10.5 Why `pyyaml` and not a lighter alternative?

The spec mandates `params.yaml` as the parameter format. PyYAML is the standard Python YAML library, well-maintained, and has no transitive dependencies. It's the obvious choice.
