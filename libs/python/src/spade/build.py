import inspect
from typing import Any, Callable, get_type_hints

from spade.types import (
    Directory,
    File,
    FileCollection,
    JsonFile,
    RasterFile,
    RasterFileCollection,
    TabularFile,
    TabularFileCollection,
    VectorFile,
    VectorFileCollection,
)

_PYTHON_TYPE_TO_MANIFEST: dict[type, dict[str, str]] = {
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

_TYPE_TO_OUTPUT_NAME: dict[type, str] = {
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


def build(fn: Callable) -> dict:
    """Generate a block manifest dict from a handler function's signature.

    Inspects the function's type hints and docstring to produce a dict
    that can be serialized to block.yaml.

    Args:
        fn: The handler function to inspect.

    Returns:
        A dict representing the block manifest (inputs, outputs, description).
    """
    hints = get_type_hints(fn)
    sig = inspect.signature(fn)

    inputs: dict[str, Any] = {}
    for param_name in sig.parameters:
        param_type = hints.get(param_name)
        if param_type is None:
            continue
        manifest_entry = _PYTHON_TYPE_TO_MANIFEST.get(param_type)
        if manifest_entry is not None:
            inputs[param_name] = dict(manifest_entry)

    outputs: dict[str, Any] = {}
    return_type = hints.get("return")
    if return_type is not None and return_type is not type(None):
        manifest_entry = _PYTHON_TYPE_TO_MANIFEST.get(return_type)
        if manifest_entry is not None:
            output_name = _TYPE_TO_OUTPUT_NAME.get(return_type, "output")
            outputs[output_name] = dict(manifest_entry)

    manifest: dict[str, Any] = {}

    if fn.__doc__:
        manifest["description"] = fn.__doc__.strip()

    manifest["inputs"] = inputs
    manifest["outputs"] = outputs

    return manifest
