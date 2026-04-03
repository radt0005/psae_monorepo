+++
title = "Types"
description = "All available Spade types in the Python library."
weight = 2
+++

The Python Spade library provides a set of types that represent the data your block works with. These types serve two purposes: they tell the library how to load inputs from the filesystem, and they map directly to manifest YAML type declarations so the runtime knows what to expect.

All types are [Pydantic](https://docs.pydantic.dev/) `BaseModel` subclasses. You use them as type hints in your handler function signature. The library inspects those hints at runtime to decide how to build the arguments it passes to your function.

## File types

File types represent single-file inputs and outputs. Each has a `path` attribute containing the filesystem path to the file.

### File

The base file type. Use this when the file does not fit into one of the more specific categories below, or when the format is not important.

```python
from spade import File

def handler(data: File) -> File:
    print(data.path)  # e.g., "inputs/data/report.pdf"
    ...
```

**Attributes:**

| Attribute | Type | Description |
|-----------|------|-------------|
| `path` | `str` | Filesystem path to the file |

**Manifest mapping:** `type: file`

### RasterFile

A file containing raster (gridded) data, such as a GeoTIFF satellite image or a digital elevation model.

```python
from spade import RasterFile

def handler(image: RasterFile) -> RasterFile:
    print(image.path)  # e.g., "inputs/image/scene.tif"
    ...
```

**Attributes:**

| Attribute | Type | Description |
|-----------|------|-------------|
| `path` | `str` | Filesystem path to the raster file |

**Manifest mapping:** `type: file`, `format: GeoTIFF`

### VectorFile

A file containing vector (geometry) data, such as a GeoJSON file or a shapefile.

```python
from spade import VectorFile

def handler(parcels: VectorFile) -> VectorFile:
    print(parcels.path)  # e.g., "inputs/parcels/boundaries.geojson"
    ...
```

**Attributes:**

| Attribute | Type | Description |
|-----------|------|-------------|
| `path` | `str` | Filesystem path to the vector file |

**Manifest mapping:** `type: file`, `format: GeoJSON`

### TabularFile

A file containing tabular data, such as a CSV or Parquet file.

```python
from spade import TabularFile

def handler(measurements: TabularFile) -> TabularFile:
    print(measurements.path)  # e.g., "inputs/measurements/data.csv"
    ...
```

**Attributes:**

| Attribute | Type | Description |
|-----------|------|-------------|
| `path` | `str` | Filesystem path to the tabular file |

**Manifest mapping:** `type: file`, `format: CSV`

### JsonFile

A file containing structured JSON data.

```python
from spade import JsonFile

def handler(config: JsonFile) -> JsonFile:
    print(config.path)  # e.g., "inputs/config/settings.json"
    ...
```

**Attributes:**

| Attribute | Type | Description |
|-----------|------|-------------|
| `path` | `str` | Filesystem path to the JSON file |

**Manifest mapping:** `type: json`

## Directory type

### Directory

Represents a directory of related files. Use this when your input is a set of files that belong together (for example, a shapefile which consists of `.shp`, `.dbf`, `.shx`, and `.prj` files).

```python
from spade import Directory

def handler(shapefile: Directory) -> Directory:
    print(shapefile.path)  # e.g., "inputs/shapefile"
    ...
```

**Attributes:**

| Attribute | Type | Description |
|-----------|------|-------------|
| `path` | `str` | Filesystem path to the directory |

**Manifest mapping:** `type: directory`

## Collection types

Collection types represent a variable-length sequence of files. The number of files is not known at pipeline design time -- it is determined at runtime (often by a map block). Each collection type has a `paths` attribute containing a list of filesystem paths.

When the Spade runtime delivers a collection input, the files are placed in a directory with zero-padded numeric filenames: `inputs/<name>/001.tif`, `inputs/<name>/002.tif`, and so on. The library scans that directory and populates the `paths` list for you.

### FileCollection

A collection of generic files.

```python
from spade import FileCollection

def handler(reports: FileCollection) -> FileCollection:
    for path in reports.paths:
        print(path)
    ...
```

**Attributes:**

| Attribute | Type | Description |
|-----------|------|-------------|
| `paths` | `list[str]` | List of filesystem paths to the files |

**Manifest mapping:** `type: collection`, `item_type: file`

### RasterFileCollection

A collection of raster data files.

```python
from spade import RasterFileCollection

def handler(tiles: RasterFileCollection) -> RasterFileCollection:
    for path in tiles.paths:
        print(path)
    ...
```

**Attributes:**

| Attribute | Type | Description |
|-----------|------|-------------|
| `paths` | `list[str]` | List of filesystem paths to the raster files |

**Manifest mapping:** `type: collection`, `item_type: file`, `format: GeoTIFF`

### VectorFileCollection

A collection of vector data files.

```python
from spade import VectorFileCollection

def handler(layers: VectorFileCollection) -> VectorFileCollection:
    for path in layers.paths:
        print(path)
    ...
```

**Attributes:**

| Attribute | Type | Description |
|-----------|------|-------------|
| `paths` | `list[str]` | List of filesystem paths to the vector files |

**Manifest mapping:** `type: collection`, `item_type: file`, `format: GeoJSON`

### TabularFileCollection

A collection of tabular data files.

```python
from spade import TabularFileCollection

def handler(sheets: TabularFileCollection) -> TabularFileCollection:
    for path in sheets.paths:
        print(path)
    ...
```

**Attributes:**

| Attribute | Type | Description |
|-----------|------|-------------|
| `paths` | `list[str]` | List of filesystem paths to the tabular files |

**Manifest mapping:** `type: collection`, `item_type: file`, `format: CSV`

## Scalar types

Scalar types represent simple values provided through the pipeline's `args` field. They are delivered to your block via `params.yaml`. You use standard Python built-in types for these -- no special Spade type is needed.

### str

A text value. Use this for configuration strings such as coordinate reference system identifiers, column names, or file format labels.

```python
def handler(target_crs: str) -> ...:
    print(target_crs)  # e.g., "EPSG:4326"
```

**Manifest mapping:** `type: string`

### int

An integer value. Use this for counts, indices, zoom levels, or other whole-number parameters.

```python
def handler(zoom: int) -> ...:
    print(zoom)  # e.g., 14
```

**Manifest mapping:** `type: number`

### float

A floating-point value. Use this for thresholds, resolutions, confidence scores, or other decimal parameters.

```python
def handler(resolution: float) -> ...:
    print(resolution)  # e.g., 30.0
```

**Manifest mapping:** `type: number`

Note that both `int` and `float` map to `type: number` in the manifest. The distinction between integer and floating-point is only enforced on the Python side.

### bool

A true/false value. Use this for feature flags or toggles.

```python
def handler(include_stats: bool) -> ...:
    print(include_stats)  # e.g., True
```

**Manifest mapping:** `type: boolean`

## Complete manifest mapping reference

This table summarizes how every Python type maps to manifest YAML fields:

| Python type | Manifest `type` | Manifest `format` | Manifest `item_type` |
|-------------|------------------|--------------------|----------------------|
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

## Inheritance hierarchy

All file and collection types are Pydantic `BaseModel` subclasses. The inheritance tree is:

```
BaseModel
  ├── File (path: str)
  │     ├── RasterFile
  │     ├── VectorFile
  │     ├── TabularFile
  │     └── JsonFile
  ├── Directory (path: str)
  └── FileCollection (paths: list[str])
        ├── RasterFileCollection
        ├── VectorFileCollection
        └── TabularFileCollection
```

The specialized subtypes (`RasterFile`, `VectorFile`, etc.) do not add extra attributes. Their purpose is to carry semantic meaning: they tell the manifest generator which `format` field to use, and they make your code self-documenting. A reader of your handler signature immediately understands that a parameter typed as `RasterFile` expects a GeoTIFF, not a CSV.

## Choosing the right type

- If your input is a **single file**, use the most specific file type available. Use `RasterFile` for GeoTIFFs, `VectorFile` for GeoJSON or shapefiles, `TabularFile` for CSVs, and `JsonFile` for JSON. Fall back to `File` when none of the specific types apply.
- If your input is a **directory** of related files that must stay together, use `Directory`.
- If your input is a **variable-length list of files** (typically from a map block), use the appropriate collection type.
- If your input is a **simple value** (a string, number, or boolean), use the corresponding Python built-in type. These values come from `params.yaml` rather than the `inputs/` directory.
