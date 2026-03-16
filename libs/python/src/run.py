import json
from typing import Callable


def scan_directory_for_inputs() -> dict:
    """Scans the current directory for input files

    Returns:
        dict: _description_
    """    
    output = {}

    return output



def run(fn: Callable):
    """Runs the user-specified function as a block

    Args:
        fn (Callable): _description_
    """
    with open("./params.yml", "r") as file:

        data = json.load(file)

    function_args = data + scan_directory_for_inputs()

    output = fn(**function_args)

    print(output)

    return 



