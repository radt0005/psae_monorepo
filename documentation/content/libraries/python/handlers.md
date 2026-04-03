+++
title = "Handler Functions"
description = "Writing handler functions with type hints for Spade blocks."
weight = 3
+++

A handler function is the core of every Python Spade block. It is a regular Python function that receives inputs and parameters as keyword arguments, does its work, and returns a result. The Spade library handles everything else: reading `params.yaml`, scanning the `inputs/` directory, building typed arguments, calling your function, and writing outputs.

## How the library calls your handler

When you call `run(handler)`, the library performs these steps in order:

1. **Load scalar parameters** from `params.yaml` into a dictionary.
2. **Scan the `inputs/` directory** and build typed objects based on your function's type hints.
3. **Merge the two dictionaries** with the expression `params | inputs`. This means that if a parameter name in `params.yaml` matches an input directory name, the file-based input takes precedence.
4. **Filter the merged dictionary** to only include keys that match your function's parameter names (unless your function accepts `**kwargs`, in which case everything is passed through).
5. **Call your function** with the filtered keyword arguments.
6. **Write the return value** to the `outputs/` directory.

## Function signature basics

Your handler's parameter names must match the names declared in your block manifest. Type hints tell the library how to construct each argument.

```python
from spade import run, RasterFile, JsonFile


def handler(raster: RasterFile, target_crs: str, resolution: float) -> JsonFile:
    """Reproject a raster and write metadata."""
    ...


if __name__ == "__main__":
    run(handler)
```

In this example:

- `raster` is a file-based input. The library looks for a directory at `inputs/raster/`, finds the file inside it, and passes a `RasterFile(path="inputs/raster/scene.tif")` object.
- `target_crs` is a scalar parameter. The library reads it from `params.yaml` and passes the string value directly.
- `resolution` is another scalar parameter, read from `params.yaml` as a float.

The names `raster`, `target_crs`, and `resolution` must appear in the block manifest as input or parameter names.

## Returning a single output

The simplest pattern is to return a single typed object. The library uses the return type to determine the output name and writes the file to the `outputs/` directory.

```python
from spade import run, TabularFile, JsonFile
import csv
import json


def handler(data: TabularFile, column: str) -> JsonFile:
    """Compute summary statistics for a CSV column."""
    values = []
    with open(data.path) as f:
        for row in csv.DictReader(f):
            try:
                values.append(float(row[column]))
            except (ValueError, KeyError):
                continue

    stats = {
        "column": column,
        "count": len(values),
        "mean": sum(values) / len(values) if values else 0,
        "min": min(values) if values else 0,
        "max": max(values) if values else 0,
    }

    output_path = "outputs/stats/summary.json"
    with open(output_path, "w") as f:
        json.dump(stats, f, indent=2)

    return JsonFile(path=output_path)


if __name__ == "__main__":
    run(handler)
```

When you return a single object, the library determines the output name in two ways:

1. If the block manifest declares exactly one output, that output's name is used regardless of the return type.
2. Otherwise, the library infers a default name from the type (for example, `JsonFile` maps to `"json"`, `RasterFile` maps to `"raster"`).

The default output names for each type are:

| Return type | Default output name |
|-------------|---------------------|
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

## Returning multiple outputs

When your block produces more than one output, return a dictionary. The keys must match the output names declared in your manifest.

```python
from spade import run, RasterFile, JsonFile


def handler(image: RasterFile, threshold: float) -> dict:
    """Classify an image and produce both a classified raster and a report."""
    from osgeo import gdal
    import json
    import numpy as np

    ds = gdal.Open(image.path)
    band = ds.GetRasterBand(1).ReadAsArray()

    # Simple threshold classification
    classified = (band > threshold).astype(np.uint8)

    # Write classified raster
    driver = gdal.GetDriverByName("GTiff")
    out_ds = driver.Create(
        "outputs/classified/result.tif",
        ds.RasterXSize, ds.RasterYSize, 1, gdal.GDT_Byte,
    )
    out_ds.SetGeoTransform(ds.GetGeoTransform())
    out_ds.SetProjection(ds.GetProjection())
    out_ds.GetRasterBand(1).WriteArray(classified)
    out_ds.FlushCache()
    out_ds = None

    # Write report
    report = {
        "threshold": threshold,
        "pixels_above": int(classified.sum()),
        "pixels_below": int((1 - classified).sum()),
        "total_pixels": int(classified.size),
    }
    with open("outputs/report/classification_report.json", "w") as f:
        json.dump(report, f, indent=2)

    return {
        "classified": RasterFile(path="outputs/classified/result.tif"),
        "report": JsonFile(path="outputs/report/classification_report.json"),
    }


if __name__ == "__main__":
    run(handler)
```

The corresponding manifest would declare both outputs:

```yaml
outputs:
  classified:
    type: file
    format: GeoTIFF
    description: Classified raster with binary class values
  report:
    type: json
    description: JSON report with classification statistics
```

## Returning None

If your block performs a side effect (such as writing to a database or uploading to an external service) and does not produce file outputs, return `None` or omit the return statement entirely.

```python
from spade import run, TabularFile
import csv


def handler(data: TabularFile, destination: str) -> None:
    """Upload CSV data to an external database."""
    import psycopg2

    conn = psycopg2.connect(destination)
    cur = conn.cursor()
    with open(data.path) as f:
        reader = csv.DictReader(f)
        for row in reader:
            cur.execute(
                "INSERT INTO measurements (station, value) VALUES (%s, %s)",
                (row["station"], float(row["value"])),
            )
    conn.commit()
    conn.close()


if __name__ == "__main__":
    run(handler)
```

When the handler returns `None`, the library skips the output-writing step entirely. The block manifest should still declare `outputs: {}` (an empty map) to signal that no outputs are produced.

Note that blocks returning `None` typically need `network: true` in their manifest if they communicate with external services.

## Accepting **kwargs

If your handler accepts `**kwargs`, the library passes the entire merged dictionary of parameters and inputs without filtering. This is useful when parameter names are not known at development time or when you want to forward everything.

```python
from spade import run, JsonFile
import json


def handler(**kwargs) -> JsonFile:
    """Echo all received parameters and inputs as JSON."""
    output = {}
    for key, value in kwargs.items():
        if hasattr(value, "path"):
            output[key] = {"type": "file", "path": value.path}
        elif hasattr(value, "paths"):
            output[key] = {"type": "collection", "paths": value.paths}
        else:
            output[key] = value

    output_path = "outputs/echo/params.json"
    with open(output_path, "w") as f:
        json.dump(output, f, indent=2)

    return JsonFile(path=output_path)


if __name__ == "__main__":
    run(handler)
```

When `**kwargs` is present, the library does not filter the merged dictionary. Every key from both `params.yaml` and `inputs/` is passed through.

## Error handling

To signal that your block has failed, raise an exception. Any unhandled exception causes the Python process to exit with a non-zero exit code, which Spade treats as a block failure. The pipeline halts immediately and the exception traceback is captured in `logs/stderr.log`.

```python
from spade import run, TabularFile, JsonFile


def handler(data: TabularFile, column: str) -> JsonFile:
    """Summarize a column, failing if the column does not exist."""
    import csv

    with open(data.path) as f:
        reader = csv.DictReader(f)
        headers = reader.fieldnames

    if column not in headers:
        raise ValueError(
            f"Column '{column}' not found in input file. "
            f"Available columns: {', '.join(headers)}"
        )

    # ... rest of the processing
```

Best practices for error handling:

- **Raise specific, descriptive exceptions.** A message like `"Column 'temperature' not found"` is far more helpful than a generic `"Invalid input"`.
- **Validate inputs early.** Check that files exist, that required columns are present, and that parameter values are within expected ranges before starting expensive computation.
- **Let unexpected errors propagate.** Do not wrap your entire handler in a try/except that swallows all errors. The library and runtime are designed to handle crashes gracefully.
- **Use standard Python exceptions.** `ValueError` for bad input data, `FileNotFoundError` for missing files, `TypeError` for wrong types. There is no need for Spade-specific exception classes.

## Working with collections

When your handler receives a collection type, it gets a list of file paths. You typically iterate over them:

```python
from spade import run, RasterFileCollection, RasterFile


def handler(tiles: RasterFileCollection) -> RasterFile:
    """Mosaic a collection of raster tiles into a single file."""
    from osgeo import gdal

    vrt = gdal.BuildVRT("temp.vrt", tiles.paths)
    gdal.Translate("outputs/mosaic/result.tif", vrt, format="GTiff")
    vrt = None

    return RasterFile(path="outputs/mosaic/result.tif")


if __name__ == "__main__":
    run(handler)
```

The `tiles.paths` list is sorted alphabetically, which preserves the zero-padded numeric ordering the runtime uses (`001.tif`, `002.tif`, ...).

## Writing output files

Your handler is responsible for writing output files to the correct locations under `outputs/`. The general pattern is:

1. Create the output subdirectory: `outputs/<output_name>/`
2. Write your file(s) into that subdirectory
3. Return a typed object pointing to the written file

```python
import os
from spade import RasterFile

# Ensure the output directory exists
os.makedirs("outputs/reprojected", exist_ok=True)

# Write the file
output_path = "outputs/reprojected/result.tif"
# ... write data to output_path ...

# Return a typed reference
return RasterFile(path=output_path)
```

The library's output writer will copy or move the file into the final output location if needed. You can write to any path under `outputs/` -- the library handles the rest.

## Complete handler template

Here is a template you can use as a starting point for new handlers:

```python
from spade import run, File, JsonFile


def handler(input_file: File, param_one: str, param_two: float) -> JsonFile:
    """Short description of what this block does."""
    import json

    # 1. Read and validate inputs
    # input_file.path contains the path to the input file
    # param_one and param_two are scalar values from params.yaml

    # 2. Process
    result = {"param_one": param_one, "param_two": param_two}

    # 3. Write outputs
    output_path = "outputs/result/output.json"
    import os
    os.makedirs("outputs/result", exist_ok=True)
    with open(output_path, "w") as f:
        json.dump(result, f, indent=2)

    # 4. Return typed reference
    return JsonFile(path=output_path)


if __name__ == "__main__":
    run(handler)
```
