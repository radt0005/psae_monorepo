# Spade Python Library

The Python library for authoring [Spade](../../spec/core.md) blocks. Define a handler function, call `run()`, and the library handles parameter loading, input discovery, and output writing.

## Installation

```bash
uv sync
```

## Quick Start

```python
from spade import run, RasterFile

def handler(source: RasterFile, buffer_distance: int) -> RasterFile:
    # Your processing logic here
    result_path = process(source.path, buffer_distance)
    return RasterFile(path=result_path)

if __name__ == "__main__":
    run(handler)
```

That's it. The `run()` function:

1. Loads scalar parameters from `params.yaml`
2. Scans `inputs/` for file-based arguments, matched by name to your function parameters
3. Calls your handler with the combined arguments
4. Writes the return value to `outputs/`

## Types

Use type hints on your handler to control how inputs are loaded and outputs are written.

### File Types

| Type | Description |
|------|-------------|
| `File` | Generic single file |
| `RasterFile` | Raster data (e.g., GeoTIFF) |
| `VectorFile` | Vector data (e.g., GeoJSON) |
| `TabularFile` | Tabular data (e.g., CSV) |
| `JsonFile` | JSON data |

Each has a `path: str` field pointing to the file location.

### Directory Type

| Type | Description |
|------|-------------|
| `Directory` | Directory-based input (e.g., shapefiles) |

Has a `path: str` field pointing to the directory.

### Collection Types

| Type | Description |
|------|-------------|
| `FileCollection` | Collection of files |
| `RasterFileCollection` | Collection of raster files |
| `VectorFileCollection` | Collection of vector files |
| `TabularFileCollection` | Collection of tabular files |

Each has a `paths: list[str]` field.

## How Inputs Are Resolved

The handler's parameter names are matched against two sources:

- **Scalar parameters** (`str`, `int`, `float`, `bool`): loaded from `params.yaml`
- **File-based inputs**: discovered from subdirectories in `inputs/`, where each subdirectory name matches a parameter name

```
params.yaml              # scalar args: {"resolution": 10, "method": "nearest"}
inputs/
  reference/
    data.tif             # -> handler(reference=RasterFile(path="inputs/reference/data.tif"))
  target/
    data.tif             # -> handler(target=RasterFile(path="inputs/target/data.tif"))
```

Type hints determine how each input is constructed:

- `File` subclass: expects a single file in the subdirectory
- `Directory`: uses the subdirectory path itself
- `FileCollection` subclass: collects all files in the subdirectory

## Output Handling

Return a typed value from your handler and `run()` writes it to `outputs/`:

```python
# Single output
def handler(source: RasterFile) -> RasterFile:
    return RasterFile(path="result.tif")

# Multiple outputs
def handler(source: RasterFile) -> dict:
    return {
        "raster": RasterFile(path="result.tif"),
        "summary": JsonFile(path="stats.json"),
    }
```

Output names are resolved from `block.yaml` (if available) or inferred from the return type.

## The `build()` Function

Generate a block manifest from a handler's signature:

```python
from spade import build, RasterFile, VectorFile

def handler(raster: RasterFile, buffer: int) -> VectorFile:
    """Converts raster boundaries to vector polygons."""
    ...

manifest = build(handler)
# {'description': 'Converts raster boundaries to vector polygons.',
#  'inputs': {'raster': {'type': 'file', 'format': 'GeoTIFF'},
#             'buffer': {'type': 'number'}},
#  'outputs': {'vector': {'type': 'file', 'format': 'GeoJSON'}}}
```

## Testing

```bash
uv run pytest
```
