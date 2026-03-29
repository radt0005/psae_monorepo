import os
import shutil
from pathlib import Path
from typing import Any

import yaml

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

_TYPE_TO_DEFAULT_NAME: dict[type, str] = {
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


def read_block_manifest() -> dict | None:
    """Attempt to read the block manifest for output declarations.

    Checks (in order):
    1. SPADE_BLOCK_MANIFEST environment variable
    2. block.yaml in the current working directory
    """
    manifest_path = os.environ.get("SPADE_BLOCK_MANIFEST")
    if manifest_path and Path(manifest_path).exists():
        with open(manifest_path, "r") as f:
            manifest = yaml.safe_load(f)
        return manifest.get("outputs") if manifest else None

    block_yaml = Path("block.yaml")
    if block_yaml.exists():
        with open(block_yaml, "r") as f:
            manifest = yaml.safe_load(f)
        return manifest.get("outputs") if manifest else None

    return None


def _infer_output_name(value: Any) -> str:
    """Infer output name from the type of the return value."""
    for type_cls, name in _TYPE_TO_DEFAULT_NAME.items():
        if type(value) is type_cls:
            return name
    return type(value).__name__.lower()


def _write_single_output(name: str, value: Any) -> None:
    """Write a single output value to outputs/<name>/."""
    output_dir = Path("outputs") / name
    output_dir.mkdir(parents=True, exist_ok=True)

    if isinstance(value, FileCollection):
        for file_path in value.paths:
            src = Path(file_path)
            shutil.copy2(src, output_dir / src.name)
    elif isinstance(value, Directory):
        src = Path(value.path)
        for item in src.iterdir():
            if item.is_file():
                shutil.copy2(item, output_dir / item.name)
            elif item.is_dir():
                shutil.copytree(item, output_dir / item.name)
    elif isinstance(value, File):
        src = Path(value.path)
        shutil.copy2(src, output_dir / src.name)


def write_outputs(result: Any, manifest_outputs: dict | None = None) -> None:
    """Write handler return value(s) to the outputs/ directory."""
    if result is None:
        return

    Path("outputs").mkdir(exist_ok=True)

    if isinstance(result, dict):
        for name, value in result.items():
            _write_single_output(name, value)
    elif isinstance(result, (File, Directory, FileCollection)):
        if manifest_outputs and len(manifest_outputs) == 1:
            name = next(iter(manifest_outputs))
        else:
            name = _infer_output_name(result)
        _write_single_output(name, result)
