+++
title = "Manifest Generation"
description = "Auto-generating block manifests from Python type hints."
weight = 4
+++

Writing block manifests by hand is straightforward for simple blocks, but as your collection grows it becomes tedious to keep the manifest YAML and the handler code in sync. The Python library provides a `build()` function that generates a manifest dictionary from your handler function's type hints and docstring. This keeps the manifest and the implementation as a single source of truth.

## The build() function

`build()` takes a handler function and returns a Python dictionary representing the block manifest. You can then serialize this to YAML and write it to `blocks/<name>.yaml`.

```python
from spade import build


def handler(raster: RasterFile, target_crs: str, resolution: float) -> RasterFile:
    """Reprojects a raster file to a target coordinate reference system."""
    ...


manifest = build(handler)
```

## What build() inspects

The `build()` function uses Python's `typing.get_type_hints()` and `inspect.signature()` to extract information from your handler:

### 1. Parameter type hints become inputs

Each parameter in the function signature is looked up in the type mapping table. If the type is recognized (any Spade type or a Python scalar like `str`, `int`, `float`, `bool`), it becomes an entry in the manifest's `inputs` section.

For example, this signature:

```python
def handler(image: RasterFile, threshold: float, label: str) -> JsonFile:
    ...
```

Produces these inputs:

```yaml
inputs:
  image:
    type: file
    format: GeoTIFF
  threshold:
    type: number
  label:
    type: string
```

Parameters without type hints are silently skipped.

### 2. Return type hint becomes the output

The function's return type annotation is mapped using the same type table. The output name is inferred from the type:

| Return type | Generated output name |
|-------------|----------------------|
| `File` | `file` |
| `RasterFile` | `raster` |
| `VectorFile` | `vector` |
| `TabularFile` | `tabular` |
| `JsonFile` | `json` |
| `Directory` | `directory` |
| `FileCollection` | `files` |
| `RasterFileCollection` | `rasters` |
| `VectorFileCollection` | `vectors` |
| `TabularFileCollection` | `tables` |

For example, a return type of `JsonFile` produces:

```yaml
outputs:
  json:
    type: json
```

If the return type is `None` or is absent, the `outputs` section will be empty.

### 3. Docstring becomes the description

If the function has a docstring, `build()` strips leading and trailing whitespace and uses it as the manifest's `description` field.

```python
def handler(data: TabularFile) -> JsonFile:
    """Compute summary statistics for a CSV file."""
    ...
```

Produces:

```yaml
description: Compute summary statistics for a CSV file.
```

If the function has no docstring, the `description` field is omitted from the generated manifest.

## Complete example

Here is a handler and the full manifest that `build()` generates from it:

### Handler

```python
from spade import RasterFile, JsonFile


def handler(
    image: RasterFile,
    target_crs: str,
    confidence_threshold: float,
    include_probabilities: bool,
) -> JsonFile:
    """Classify land cover in a satellite image using a pre-trained model."""
    ...
```

### Generated manifest (as Python dict)

```python
from spade import build

manifest = build(handler)
print(manifest)
```

```python
{
    "description": "Classify land cover in a satellite image using a pre-trained model.",
    "inputs": {
        "image": {"type": "file", "format": "GeoTIFF"},
        "target_crs": {"type": "string"},
        "confidence_threshold": {"type": "number"},
        "include_probabilities": {"type": "boolean"},
    },
    "outputs": {
        "json": {"type": "json"},
    },
}
```

### Equivalent YAML

When serialized to YAML, this produces:

```yaml
description: Classify land cover in a satellite image using a pre-trained model.

inputs:
  image:
    type: file
    format: GeoTIFF
  target_crs:
    type: string
  confidence_threshold:
    type: number
  include_probabilities:
    type: boolean

outputs:
  json:
    type: json
```

Note that `build()` generates only the `description`, `inputs`, and `outputs` fields. You still need to add the top-level metadata fields yourself:

```yaml
id: ml.classify
version: 1.0.0
kind: standard
network: false
entrypoint: src/ml/classify.py
```

These fields depend on your collection structure and deployment decisions, so the library does not attempt to infer them.

## Using build() in a script

A common pattern is to create a small script that generates manifests for all blocks in your collection:

```python
#!/usr/bin/env python3
"""Generate block manifests from handler type hints."""

import yaml
from pathlib import Path

from spade import build

# Import your handlers
from my_collection.reproject import handler as reproject_handler
from my_collection.summarize import handler as summarize_handler


BLOCKS = {
    "reproject": {
        "handler": reproject_handler,
        "id": "my-collection.reproject",
        "version": "0.1.0",
        "kind": "standard",
        "network": False,
        "entrypoint": "src/my_collection/reproject.py",
    },
    "summarize": {
        "handler": summarize_handler,
        "id": "my-collection.summarize",
        "version": "0.1.0",
        "kind": "standard",
        "network": False,
        "entrypoint": "src/my_collection/summarize.py",
    },
}


def main():
    blocks_dir = Path("blocks")
    blocks_dir.mkdir(exist_ok=True)

    for name, config in BLOCKS.items():
        handler = config.pop("handler")
        generated = build(handler)

        # Merge static metadata with generated inputs/outputs
        manifest = {**config, **generated}

        output_path = blocks_dir / f"{name}.yaml"
        with open(output_path, "w") as f:
            yaml.dump(manifest, f, default_flow_style=False, sort_keys=False)

        print(f"  Generated {output_path}")


if __name__ == "__main__":
    main()
```

Run this script whenever you change a handler's signature or docstring:

```bash
python generate_manifests.py
```

This approach ensures that the manifest always reflects the actual code. If you add a parameter to your handler, the manifest is updated automatically the next time you run the generation script.

## Type mapping reference

The complete mapping from Python types to manifest fields, used by both `build()` and `run()`:

| Python type | `type` | `format` | `item_type` |
|-------------|--------|----------|-------------|
| `File` | `file` | -- | -- |
| `RasterFile` | `file` | `GeoTIFF` | -- |
| `VectorFile` | `file` | `GeoJSON` | -- |
| `TabularFile` | `file` | `CSV` | -- |
| `JsonFile` | `json` | -- | -- |
| `Directory` | `directory` | -- | -- |
| `FileCollection` | `collection` | -- | `file` |
| `RasterFileCollection` | `collection` | `GeoTIFF` | `file` |
| `VectorFileCollection` | `collection` | `GeoJSON` | `file` |
| `TabularFileCollection` | `collection` | `CSV` | `file` |
| `str` | `string` | -- | -- |
| `int` | `number` | -- | -- |
| `float` | `number` | -- | -- |
| `bool` | `boolean` | -- | -- |

## Limitations

- **`build()` does not generate `id`, `version`, `kind`, `network`, or `entrypoint` fields.** These depend on your project structure and cannot be inferred from the function alone.
- **`dict` return types are not introspected.** If your handler returns a dictionary of multiple outputs, `build()` cannot determine the individual output types. You will need to add the `outputs` section to the manifest manually.
- **Parameter descriptions are not generated.** The `description` field on individual inputs and outputs is not extracted from the docstring. You may want to add these by hand for better documentation.
- **Unrecognized types are skipped.** If a parameter has a type hint that is not in the mapping table (for example, `Optional[str]` or a custom class), `build()` ignores it. Use only the types listed in the mapping table above.
