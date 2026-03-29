import inspect
from typing import Callable

from spade._output import read_block_manifest, write_outputs
from spade._scanning import build_function_args


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
    args = build_function_args(fn)

    sig = inspect.signature(fn)
    param_names = set(sig.parameters.keys())

    has_var_keyword = any(
        p.kind == inspect.Parameter.VAR_KEYWORD
        for p in sig.parameters.values()
    )

    if has_var_keyword:
        filtered_args = args
    else:
        filtered_args = {k: v for k, v in args.items() if k in param_names}

    result = fn(**filtered_args)

    manifest_outputs = read_block_manifest()
    write_outputs(result, manifest_outputs)
