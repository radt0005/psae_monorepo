# Spade Libraries

The runtime library exposes types and functions for running the blocks.  To make a block, the user only has to define a function (or, in Python, a `Callable`) and pass it to the `run()` function. 

There are a number of helper methods and testing utilities included in this as well.  


## Types

The library defines some custom classes (or, in R, S4 objects) for inputs and outputs, and the `run` function that handles running the block when called from the runtime.  An example of how this works is in the examples section. 

These correspond to the output types from the block specification.  This way, users can use them in blocks to have typed inputs in the handler function and use specific file formats or types for the return.  

For example, users could do something like this in Python:

```python

from spade import RasterFile, RasterFileCollection, VectorFile

def handler(base_image: RasterFile, polygons: VectorFile) -> RasterFileCollection:

    # user provided code to do something useful

    return outputs

```

### Type List

The types should be created from base classes representing a file or directory.  These all wrap the same base model and have a single field, path, that has the file path for loading the file.  This way the user never has to worry about the actual paths to the file.  For a directory or collection, it has a field "paths" that is a list of paths.  

For example, in Python, this would look like this: 
```python

class File(BaseModel):
    path: str 

class Directory(BaseModel):
    path: str

```

Then, each could have a further type specifier so that the user could specify which file type is used  For example, 

```python
class RasterFile(File):
    """Represents a Raster Data file (e.g. GeoTiff)
    """    
    pass

class TabularFile(File):
    pass

class VectorFile(File):
    pass

```

There should be types for directories of rasters, vectors, and tables as well.  


## Making a Block (User Journey)

To create a block, the user defines a function, and then calls the run method taking the handler function as an argument.  That is all that is required.  In order to have typed arguments, there are specified types for arguments in the library to make working with them easier. 

The user begins by using the CLI to initialize the repository. This sets up the infrastructure for the block collection.  Then the user can add blocks to the collection using the CLI as well (which handles the boilerplate).  

Then the user defines a function that does the processing they want the block to do. Finally they call the run() function, passing their user-defined function as an argument.  

When the block is called, the run function loads the parameters from the file system in the params.yaml file, and then scans the `./inputs` directory for folders.  Each directory name corresponds to the name of a function input, so that is used as the name of the file.  This means that for a function input name "raster", the file the function needs is located at `./inputs/raster/*`, and there should only be one file in that directory.  

Once the working directory is scanned for files, the system should be able to call the hanlder function.  In Python, this would look something like this: 

```python
import yaml
from typing import Callable, Any

def scan_working_directory() -> dict:

    output = {}
    # code to scan the working directory and put them in the dictionary by parameter name
    return output

with open("./params.yaml", "r") as file: 
    params = yaml.safe_load(file)

function_args = params | scan_working_directory()

```

Then the handler function can be called by just unpacking the `function_args` dictionary.  Assuming the handler is called `handler`, we can just call it with this: `handler(**function_args)`.  This should allow for efficient calling of functions.  

### Python

An example of a "Hello World" block might look like this: 

```python

from spade import run, File

# the user-defined function
def handler(file: File):
    print("Hello World")



# include guard so handler is importable
if __name__ == "__main__":
    run(handler)
```

In other languages this should be similar.  In Rust and Go (or other compiled languages) this will look slightly different, but the idea is the same (for example, rust might need to use a closure instead of a function).  Also, another variation in TypeScript may need to expose `run()` as an async function.

## Output Handling

The `run()` function is responsible for writing the handler's return value to the `outputs/` directory.  After the handler function returns, the library inspects the return value and writes each named output into a subdirectory of `outputs/` matching the output name declared in `block.yaml`.

For example, if the handler returns a `RasterFile`, the `run()` function copies (or moves) the file at the returned path into `outputs/<output_name>/`.  For collections, each item is written into the output subdirectory.

This means block authors only need to return the appropriate typed value from their handler -- the library handles the filesystem layout automatically.

```python
from spade import run, RasterFile

def handler(source: RasterFile) -> RasterFile:
    # process the raster and save result to a temporary path
    result_path = process(source.path)
    return RasterFile(path=result_path)

if __name__ == "__main__":
    run(handler)
```

In this example, `run()` would take the returned `RasterFile` and copy the file at `result_path` into `outputs/raster/` (where `raster` is the output name from `block.yaml`).

## Supported Languages

The library for writing function should be available in the following languages:
1. Python
2. R
3. TypeScript
4. Go
5. Rust



## Collection Development Workflow

Block collections are developed using standard language toolchains.  There is no separate build step in Spade -- compilation and packaging use the language's native tools (`cargo build`, `go build`, `bun build`, etc.).

The CLI provides scaffolding and validation:
- `spade init` creates the collection structure for a chosen language
- `spade add <name>` scaffolds a new block (manifest + entry point)
- `spade check` validates all block manifests in the collection
- `spade upload` packages and uploads the collection for cloud deployment

See `cli.md` for full details on each command.
