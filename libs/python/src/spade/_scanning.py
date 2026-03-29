from pathlib import Path
from typing import Any, Callable, get_type_hints

import yaml

from spade.types import (
    Directory,
    File,
    FileCollection,
)


def load_params() -> dict:
    """Load scalar parameters from params.yaml."""
    params_path = Path("params.yaml")
    if not params_path.exists():
        return {}
    with open(params_path, "r") as f:
        params = yaml.safe_load(f)
    return params if params is not None else {}


def scan_inputs(type_hints: dict[str, type]) -> dict[str, Any]:
    """Scan the inputs/ directory and build typed arguments.

    Args:
        type_hints: Mapping of parameter name -> expected type from the
                    handler's annotations.

    Returns:
        Dict mapping parameter name -> typed value.
    """
    inputs_dir = Path("inputs")
    if not inputs_dir.exists():
        return {}

    result = {}
    for subdir in sorted(inputs_dir.iterdir()):
        if not subdir.is_dir():
            continue

        name = subdir.name
        expected_type = type_hints.get(name)

        files = sorted([f for f in subdir.iterdir() if f.is_file()])

        if expected_type is not None:
            try:
                if issubclass(expected_type, Directory):
                    result[name] = expected_type(path=str(subdir))
                    continue
                elif issubclass(expected_type, FileCollection):
                    result[name] = expected_type(paths=[str(f) for f in files])
                    continue
                elif issubclass(expected_type, File):
                    if not files:
                        raise ValueError(
                            f"Input '{name}' expects a file but directory "
                            f"'{subdir}' is empty"
                        )
                    result[name] = expected_type(path=str(files[0]))
                    continue
            except TypeError:
                pass  # expected_type is not a class, fall through

        # Default: infer from file count
        if not files:
            raise ValueError(f"Input directory '{subdir}' is empty")
        if len(files) == 1:
            result[name] = File(path=str(files[0]))
        else:
            result[name] = FileCollection(paths=[str(f) for f in files])

    return result


def build_function_args(fn: Callable) -> dict[str, Any]:
    """Build the full arguments dict for the handler function.

    Merges scalar parameters from params.yaml with file-based inputs
    from the inputs/ directory. Inputs take precedence over params.
    """
    type_hints = get_type_hints(fn)
    type_hints.pop("return", None)

    params = load_params()
    inputs = scan_inputs(type_hints)

    return params | inputs
