# Type and Format Reference

This document is the lookup table for **block manifest types** (the `type:` field in `block.yaml`) and the **runtime library types** (the typed values block authors use in handlers). They line up — the runtime types are language bindings of the manifest types.

Use this when deciding what to put in a manifest, picking the right type for a handler argument, or setting `format` consistently.

---

## Manifest types (the `type:` field)

These are the values that go in `inputs.<name>.type` and `outputs.<name>.type` in `block.yaml`.

### Input types

| `type:`     | Description                                                                  |
| ----------- | ---------------------------------------------------------------------------- |
| `file`      | A single file. Symlinked into `inputs/<name>/`.                              |
| `directory` | A directory. The whole subdirectory at `inputs/<name>/` is the input.        |
| `collection`| A variable-length collection of items. Requires `item_type:` to specify each item's type. Stored as multiple files under `inputs/<name>/`. |
| `string`    | Scalar string parameter. Delivered via `params.yaml`, **not** the `inputs/` directory. |
| `number`    | Scalar numeric parameter (int or float). Delivered via `params.yaml`.        |
| `boolean`   | Scalar boolean parameter. Delivered via `params.yaml`.                        |

### Output types

| `type:`     | Description                                                                  |
| ----------- | ---------------------------------------------------------------------------- |
| `file`      | A single file written into `outputs/<name>/`.                                |
| `directory` | A whole directory written into `outputs/<name>/`.                            |
| `collection`| A variable-length collection of items. Requires `item_type:`.                |
| `json`      | A JSON file written into `outputs/<name>/`. Use this for structured metadata, statistics, summaries. |
| `expansion` | **Only valid on `kind: map` blocks.** The map expansion manifest at `outputs/<name>/expansion.yaml`. |

### `format` field

`format` is a free-form hint describing the file format. It's not enforced by the runtime but is shown in the UI and used by `spade check` for type compatibility checks. Use these conventional values:

| `format:`     | Typical extensions      | Notes                                  |
| ------------- | ----------------------- | -------------------------------------- |
| `GeoTIFF`     | `.tif`, `.tiff`         | Raster imagery                         |
| `COG`         | `.tif`                  | Cloud Optimized GeoTIFF                |
| `VRT`         | `.vrt`                  | GDAL Virtual Raster                    |
| `NetCDF`      | `.nc`                   | Multidimensional array data            |
| `GeoJSON`     | `.geojson`, `.json`     | Vector geometries                      |
| `Shapefile`   | `.shp` (+ siblings)     | Vector — use `type: directory`         |
| `CSV`         | `.csv`                  | Tabular data                           |
| `Parquet`     | `.parquet`              | Columnar tabular data                  |
| `GeoParquet`  | `.parquet`              | Geospatial columnar data               |
| `JSON`        | `.json`                 | Generic JSON (use `type: json`)        |

When in doubt, set `format` — it makes blocks self-documenting and improves type matching in the pipeline resolver.

### `item_type` field

For `type: collection`, `item_type` says what each item in the collection is. The most common value is `file` (a collection of files). Combine with `format` to describe the file format of each item:

```yaml
inputs:
  tiles:
    type: collection
    item_type: file
    format: GeoTIFF
    description: Collection of raster tiles
```

---

## Runtime library types

These are the typed values used in handler signatures. Each language ships the same set, named to match the manifest types where possible. They are documented here together — pick the row for the language you're working in.

### File / directory types

| Manifest `type` + `format` | Python              | R              | Rust          | Go             | TypeScript     |
| -------------------------- | ------------------- | -------------- | ------------- | -------------- | -------------- |
| `file` (any)               | `File`              | `File()`       | `File`        | `spade.File`   | `File`         |
| `file` `GeoTIFF`           | `RasterFile`        | `RasterFile()` | `RasterFile`  | `spade.RasterFile` | `RasterFile` |
| `file` `GeoJSON`           | `VectorFile`        | `VectorFile()` | `VectorFile`  | `spade.VectorFile` | `VectorFile` |
| `file` `CSV` / `Parquet`   | `TabularFile`       | `TabularFile()`| `TabularFile` | `spade.TabularFile`| `TabularFile`|
| `json`                     | `JsonFile`          | `JsonFile()`   | `JsonFile`    | `spade.JsonFile`   | `JsonFile`   |
| `directory`                | `Directory`         | `Directory()`  | `Directory`   | `spade.Directory`  | `Directory`  |

### Collection types

| Manifest `type` + `item_type` + `format` | Python                  | R                          | Rust                    | Go                              | TypeScript                |
| ---------------------------------------- | ----------------------- | -------------------------- | ----------------------- | ------------------------------- | ------------------------- |
| `collection` `file` (any)                | `FileCollection`        | `FileCollection()`         | `FileCollection`        | `spade.FileCollection`          | `FileCollection`          |
| `collection` `file` `GeoTIFF`            | `RasterFileCollection`  | `RasterFileCollection()`   | `RasterFileCollection`  | `spade.RasterFileCollection`    | `RasterFileCollection`    |
| `collection` `file` `GeoJSON`            | `VectorFileCollection`  | `VectorFileCollection()`   | `VectorFileCollection`  | `spade.VectorFileCollection`    | `VectorFileCollection`    |
| `collection` `file` `CSV`/`Parquet`      | `TabularFileCollection` | `TabularFileCollection()`  | `TabularFileCollection` | `spade.TabularFileCollection`   | `TabularFileCollection`   |

All file types have a single field/slot:
- Python / TypeScript / Rust / Go: `path` (string)
- R (S4): `@path`

All collection types have:
- Python / TypeScript / Rust / Go: `paths` (list/slice/Vec of strings)
- R (S4): `@paths`

### Scalar parameters (from `params.yaml`)

Scalar parameters are not wrapped in a runtime type — they come through as the language's native scalar:

| Manifest `type` | Python      | R           | Rust              | Go             | TypeScript |
| --------------- | ----------- | ----------- | ----------------- | -------------- | ---------- |
| `string`        | `str`       | `character` | `String`          | `string`       | `string`   |
| `number`        | `int`/`float` | `numeric` | `f64` / `i64` etc.| `float64`/`int`| `number`   |
| `boolean`       | `bool`      | `logical`   | `bool`            | `bool`         | `boolean`  |

The runtime libraries pull scalars from `params.yaml` and pass them as keyword arguments alongside the file-based inputs. Use the same parameter name in the handler as in the manifest.

---

## Choosing a type

A few rules of thumb:

- **Single file with a known format:** use the most specific runtime type (`RasterFile`, `VectorFile`, `TabularFile`, `JsonFile`) and set `format` in the manifest. This gives good UI hints, helps the pipeline resolver match types unambiguously, and documents intent.
- **Collection of files:** use the matching `*Collection` runtime type, set `type: collection`, `item_type: file`, and `format` accordingly.
- **A whole directory** (e.g. a Shapefile bundle, a tile cache): use `Directory` and `type: directory`.
- **Structured metadata, summaries, statistics:** use `JsonFile` and `type: json`. Don't reach for `TabularFile` for these — `json` is the convention.
- **Scalar configuration:** use `string` / `number` / `boolean` and pass via `args` in the pipeline. These go through `params.yaml`, never `inputs/`.
- **A `kind: map` block's manifest output:** use `type: expansion`. This is the only place `expansion` is valid.

---

## Custom types

The base library covers the common geospatial cases. For domain-specific types not in the table above, use the generic `File` / `FileCollection` types and set `format` to describe the format. The runtime libraries don't currently support fully custom subclasses with bespoke loading logic — use `File` and read the file inside the handler.
