+++
title = "Examples"
description = "Complete worked examples of Python blocks."
weight = 5
+++

This page presents three complete examples of Python Spade blocks. Each example includes the block manifest, the full handler implementation, and an explanation of how the pieces fit together.

## Example 1: Raster reprojection

This block takes a GeoTIFF raster file and reprojects it to a different coordinate reference system (CRS) using GDAL. It accepts the target CRS and an optional output resolution as parameters.

### Manifest

`blocks/reproject.yaml`:

```yaml
id: raster.reproject
version: 0.2.1
kind: standard
network: false
description: Reprojects a raster file to a target coordinate reference system using GDAL.

entrypoint: src/raster/reproject.py

inputs:
  raster:
    type: file
    format: GeoTIFF
    description: The input raster file to reproject
  target_crs:
    type: string
    description: Target coordinate reference system (e.g., "EPSG:4326")
  resolution:
    type: number
    description: Output pixel resolution in target CRS units

outputs:
  reprojected:
    type: file
    format: GeoTIFF
    description: The reprojected raster file
```

### Handler

`src/raster/reproject.py`:

```python
import os
from spade import run, RasterFile


def handler(raster: RasterFile, target_crs: str, resolution: float) -> RasterFile:
    """Reprojects a raster file to a target coordinate reference system using GDAL."""
    from osgeo import gdal

    # Open the source raster
    src_ds = gdal.Open(raster.path)
    if src_ds is None:
        raise FileNotFoundError(f"Could not open raster file: {raster.path}")

    # Configure warp options
    warp_options = gdal.WarpOptions(
        dstSRS=target_crs,
        xRes=resolution,
        yRes=resolution,
        resampleAlg="bilinear",
        format="GTiff",
        creationOptions=["COMPRESS=LZW", "TILED=YES"],
    )

    # Ensure output directory exists
    output_path = "outputs/reprojected/result.tif"
    os.makedirs(os.path.dirname(output_path), exist_ok=True)

    # Run the reprojection
    result_ds = gdal.Warp(output_path, src_ds, options=warp_options)
    if result_ds is None:
        raise RuntimeError(
            f"GDAL Warp failed. Check that '{target_crs}' is a valid CRS "
            f"and that the input raster is not corrupted."
        )

    # Log basic info about the result
    print(f"Reprojected to {target_crs}")
    print(f"Output size: {result_ds.RasterXSize} x {result_ds.RasterYSize}")
    print(f"Output resolution: {resolution} units/pixel")

    # Close datasets
    result_ds.FlushCache()
    result_ds = None
    src_ds = None

    return RasterFile(path=output_path)


if __name__ == "__main__":
    run(handler)
```

### Explanation

- **Inputs.** The handler receives three arguments: `raster` (a `RasterFile` with a `.path` pointing to the GeoTIFF in `inputs/raster/`), `target_crs` (a string from `params.yaml`), and `resolution` (a float from `params.yaml`).
- **Processing.** GDAL's `Warp` function handles the reprojection. The handler configures bilinear resampling and LZW compression.
- **Error handling.** The handler raises `FileNotFoundError` if GDAL cannot open the input and `RuntimeError` if the warp operation fails. Both produce a non-zero exit code that halts the pipeline.
- **Output.** The reprojected file is written to `outputs/reprojected/result.tif`. The handler returns a `RasterFile` pointing to it. Because the manifest declares exactly one output named `reprojected`, the library writes the result to that output slot.
- **Logging.** Print statements go to `logs/stdout.log` and are available for debugging after the run completes.

### Pipeline usage

```yaml
blocks:
  - id: "@reproject"
    name: raster.reproject
    inputs:
      - "@source"  # upstream block producing a raster
    args:
      target_crs: "EPSG:4326"
      resolution: 30
```

---

## Example 2: CSV to JSON statistics

This block reads a CSV file, computes summary statistics for specified numeric columns, and outputs the results as a JSON file. It demonstrates working with tabular data and producing structured JSON output.

### Manifest

`blocks/csv-stats.yaml`:

```yaml
id: analysis.csv-stats
version: 1.0.0
kind: standard
network: false
description: >
  Computes descriptive statistics for numeric columns in a CSV file.
  Produces a JSON report with count, mean, median, min, max, and
  standard deviation for each requested column.

entrypoint: src/analysis/csv_stats.py

inputs:
  data:
    type: file
    format: CSV
    description: The input CSV file to analyze
  columns:
    type: string
    description: Comma-separated list of column names to summarize

outputs:
  stats:
    type: json
    description: JSON report containing computed statistics per column
```

### Handler

`src/analysis/csv_stats.py`:

```python
import csv
import json
import math
import os
from spade import run, TabularFile, JsonFile


def _median(values: list[float]) -> float:
    """Compute the median of a sorted list of values."""
    n = len(values)
    if n == 0:
        return 0.0
    sorted_vals = sorted(values)
    mid = n // 2
    if n % 2 == 0:
        return (sorted_vals[mid - 1] + sorted_vals[mid]) / 2
    return sorted_vals[mid]


def _stddev(values: list[float], mean: float) -> float:
    """Compute the population standard deviation."""
    if len(values) < 2:
        return 0.0
    variance = sum((v - mean) ** 2 for v in values) / len(values)
    return math.sqrt(variance)


def handler(data: TabularFile, columns: str) -> JsonFile:
    """Compute descriptive statistics for numeric columns in a CSV file."""

    # Parse the comma-separated column list
    requested_columns = [c.strip() for c in columns.split(",")]

    # Read the CSV file
    with open(data.path) as f:
        reader = csv.DictReader(f)
        headers = reader.fieldnames or []

        # Validate that all requested columns exist
        missing = [c for c in requested_columns if c not in headers]
        if missing:
            raise ValueError(
                f"Columns not found in CSV: {', '.join(missing)}. "
                f"Available columns: {', '.join(headers)}"
            )

        # Collect numeric values for each column
        column_values: dict[str, list[float]] = {c: [] for c in requested_columns}
        row_count = 0
        for row in reader:
            row_count += 1
            for col in requested_columns:
                try:
                    column_values[col].append(float(row[col]))
                except (ValueError, TypeError):
                    pass  # Skip non-numeric values

    # Compute statistics
    report = {
        "source_rows": row_count,
        "columns": {},
    }

    for col in requested_columns:
        values = column_values[col]
        if not values:
            report["columns"][col] = {
                "error": "No numeric values found",
                "non_numeric_count": row_count,
            }
            continue

        mean = sum(values) / len(values)
        report["columns"][col] = {
            "count": len(values),
            "mean": round(mean, 6),
            "median": round(_median(values), 6),
            "min": min(values),
            "max": max(values),
            "stddev": round(_stddev(values, mean), 6),
        }

    # Write output
    output_path = "outputs/stats/report.json"
    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    with open(output_path, "w") as f:
        json.dump(report, f, indent=2)

    print(f"Analyzed {row_count} rows across {len(requested_columns)} columns")

    return JsonFile(path=output_path)


if __name__ == "__main__":
    run(handler)
```

### Explanation

- **Inputs.** `data` is a `TabularFile` pointing to the CSV in `inputs/data/`. `columns` is a string from `params.yaml` containing a comma-separated list of column names.
- **Validation.** Before doing any computation, the handler checks that all requested columns actually exist in the CSV. If any are missing, it raises a `ValueError` with a clear message listing the available columns.
- **Processing.** The handler reads the CSV once, collecting numeric values for each requested column. It then computes count, mean, median, min, max, and standard deviation. Non-numeric values in a column are silently skipped, and the handler reports how many numeric values it found.
- **No external dependencies.** This block uses only the Python standard library (`csv`, `json`, `math`). It does not need numpy, pandas, or any other data science library. For simple statistical operations, the standard library is sufficient and avoids unnecessary dependencies.
- **Output.** The JSON report is written to `outputs/stats/report.json` and returned as a `JsonFile`.

### Sample output

For a CSV with columns `station`, `temperature`, and `humidity`, called with `columns: "temperature,humidity"`:

```json
{
  "source_rows": 365,
  "columns": {
    "temperature": {
      "count": 365,
      "mean": 15.234521,
      "median": 14.8,
      "min": -12.3,
      "max": 41.2,
      "stddev": 9.876543
    },
    "humidity": {
      "count": 360,
      "mean": 62.451233,
      "median": 64.0,
      "min": 12.0,
      "max": 100.0,
      "stddev": 18.234567
    }
  }
}
```

### Pipeline usage

```yaml
blocks:
  - id: "@csv-stats"
    name: analysis.csv-stats
    inputs:
      - "@source"  # upstream block producing a CSV
    args:
      columns: "temperature,humidity,pressure"
```

---

## Example 3: Tile enumeration map block

This block is a **map block** that takes a bounding box and zoom level, enumerates the web map tiles covering that area, downloads each tile, and writes an expansion manifest so the scheduler can fan out downstream blocks across the individual tiles.

Map blocks are the starting point of the [map/reduce](/concepts/map-reduce/) pattern. They produce an `expansion.yaml` file that tells the scheduler how many items exist and where each one is located.

### Manifest

`blocks/enumerate-tiles.yaml`:

```yaml
id: tiles.enumerate
version: 1.0.0
kind: map
network: true
description: >
  Enumerates web map tiles for a bounding box at a given zoom level.
  Downloads each tile and produces an expansion manifest for downstream
  parallel processing.

entrypoint: src/tiles/enumerate.py

inputs:
  bbox:
    type: string
    description: >
      Bounding box as "west,south,east,north" in EPSG:4326
      (e.g., "-105.5,40.0,-105.0,40.5")
  zoom:
    type: number
    description: Tile zoom level (e.g., 14)

outputs:
  tile:
    type: expansion
    format: PNG
    description: Individual map tiles for parallel processing
```

### Handler

`src/tiles/enumerate.py`:

```python
import math
import os
import json
import yaml
from urllib.request import urlretrieve
from spade import run


def _lng_to_tile_x(lng: float, zoom: int) -> int:
    """Convert longitude to tile X coordinate."""
    return int((lng + 180.0) / 360.0 * (2 ** zoom))


def _lat_to_tile_y(lat: float, zoom: int) -> int:
    """Convert latitude to tile Y coordinate."""
    lat_rad = math.radians(lat)
    n = 2 ** zoom
    return int((1.0 - math.log(math.tan(lat_rad) + 1.0 / math.cos(lat_rad)) / math.pi) / 2.0 * n)


def handler(bbox: str, zoom: int) -> None:
    """Enumerate web map tiles for a bounding box at a given zoom level."""

    # Parse bounding box
    parts = [float(x.strip()) for x in bbox.split(",")]
    if len(parts) != 4:
        raise ValueError(
            f"Expected bbox as 'west,south,east,north', got {len(parts)} values"
        )
    west, south, east, north = parts

    # Compute tile range
    x_min = _lng_to_tile_x(west, zoom)
    x_max = _lng_to_tile_x(east, zoom)
    y_min = _lat_to_tile_y(north, zoom)  # Note: y is inverted
    y_max = _lat_to_tile_y(south, zoom)

    print(f"Bounding box: {west}, {south}, {east}, {north}")
    print(f"Zoom level: {zoom}")
    print(f"Tile range: x=[{x_min},{x_max}], y=[{y_min},{y_max}]")

    # Create output directory for tiles
    tiles_dir = "outputs/tile"
    os.makedirs(tiles_dir, exist_ok=True)

    # Download tiles and build expansion items
    items = []
    tile_index = 0

    for x in range(x_min, x_max + 1):
        for y in range(y_min, y_max + 1):
            tile_index += 1
            key = f"{zoom}_{x}_{y}"
            filename = f"{key}.png"
            tile_path = os.path.join(tiles_dir, filename)

            # Download from a tile server
            url = f"https://tile.openstreetmap.org/{zoom}/{x}/{y}.png"
            try:
                urlretrieve(url, tile_path)
                print(f"  Downloaded tile {tile_index}: {key}")
            except Exception as e:
                raise RuntimeError(
                    f"Failed to download tile {key} from {url}: {e}"
                )

            items.append({
                "path": f"tile/{filename}",
                "key": key,
            })

    if not items:
        raise ValueError(
            f"No tiles found for bbox={bbox} at zoom={zoom}. "
            "Check that the bounding box coordinates are valid."
        )

    print(f"Total tiles enumerated: {len(items)}")

    # Write the expansion manifest
    expansion = {"items": items}
    expansion_path = os.path.join(tiles_dir, "expansion.yaml")
    with open(expansion_path, "w") as f:
        yaml.dump(expansion, f, default_flow_style=False)

    # Map blocks return None -- the expansion manifest is the output
    return None


if __name__ == "__main__":
    run(handler)
```

### Explanation

- **Block kind.** This block has `kind: map` in its manifest, which tells the scheduler that it produces an expansion output. The output type is `expansion`, not `file` or `json`.
- **Network access.** The block needs to download tiles from a web server, so `network: true` is set in the manifest. Most blocks should leave this as `false`.
- **Inputs.** `bbox` is a string parameter from `params.yaml` containing four comma-separated coordinates. `zoom` is an integer parameter.
- **Tile math.** The helper functions `_lng_to_tile_x` and `_lat_to_tile_y` convert geographic coordinates to the standard web map tile numbering scheme (the "slippy map" convention).
- **Expansion manifest.** The handler writes an `expansion.yaml` file inside the output directory. Each item has a `path` (the tile file relative to the output directory) and a `key` (a unique identifier for that tile). The scheduler reads this manifest and creates one downstream invocation per item.
- **Return value.** Map blocks return `None`. The expansion manifest is the actual output that drives the pipeline. The library's output writer skips the write step when the return value is `None`.
- **Error handling.** The handler validates the bounding box format, checks that at least one tile was found, and wraps download failures in a `RuntimeError` with a descriptive message.

### Generated expansion.yaml

For a small bounding box at zoom level 14, the expansion manifest might look like:

```yaml
items:
  - path: tile/14_3412_6178.png
    key: 14_3412_6178
  - path: tile/14_3412_6179.png
    key: 14_3412_6179
  - path: tile/14_3413_6178.png
    key: 14_3413_6178
  - path: tile/14_3413_6179.png
    key: 14_3413_6179
```

### Pipeline usage

In a pipeline, this map block fans out to a processing block and then a reduce block collects the results:

```yaml
blocks:
  # Step 1: Enumerate tiles (map block)
  - id: "@enumerate"
    name: tiles.enumerate
    inputs: []
    args:
      bbox: "-105.5,40.0,-105.0,40.5"
      zoom: 14

  # Step 2: Process each tile (runs N times, once per tile)
  - id: "@analyze"
    name: raster.analyze
    inputs:
      - "@enumerate"
    args:
      analysis_type: ndvi

  # Step 3: Collect results (reduce block)
  - id: "@aggregate"
    name: analysis.aggregate
    inputs:
      - "@analyze"
    args:
      method: mean
```

The scheduler automatically creates N invocations of `raster.analyze` (one per tile from the expansion manifest), runs them in parallel, and then feeds all N results into `analysis.aggregate` as a collection input.

---

## Patterns across all examples

These three examples illustrate the core patterns of Python Spade blocks:

1. **Type hints are the contract.** The function signature is the single source of truth for what the block accepts. File types come from `inputs/`, scalar types come from `params.yaml`.

2. **You write the files yourself.** The library does not provide a magic API for writing outputs. You use standard Python file I/O (or libraries like GDAL, pandas, etc.) to write to `outputs/<name>/`. You then return a typed object pointing to what you wrote.

3. **Errors are just exceptions.** Raise a standard Python exception with a clear message. The runtime captures it and halts the pipeline.

4. **`run(handler)` is the only framework call.** The two-line pattern at the bottom of every handler file -- define the function, then call `run()` -- is all the Spade-specific code you need.
