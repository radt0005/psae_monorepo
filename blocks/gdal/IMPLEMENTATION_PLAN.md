# GDAL Blocks — Implementation Plan

This plan tracks the work to implement the 47 blocks specified in `SPECIFICATION.md`. Each block's "done" bar is: (a) manifest at `blocks/<name>.yaml` with full `inputs`/`outputs` and descriptions, (b) handler at `src/gdal_blocks/<name>.py` using the spade runtime types, and (c) passes `spade check`.

---

## Phase 0 — Collection scaffolding

- [x] Run `spade init --language python` in `blocks/gdal/` to generate `pyproject.toml`, `src/gdal_blocks/__init__.py`, and the empty `blocks/` directory
- [x] Set the pyproject `name` to `gdal-blocks` (the PyPI `gdal` package and our collection would collide otherwise); collection name stays `gdal` because block IDs are explicit
- [x] Add dependencies to `pyproject.toml`: `gdal==3.10.0` (via girder wheels), `spade` (editable from `../../libs/python`), `numpy`, `pyyaml`, `pytest` (dev)
- [x] Configure `no-build-package = ["gdal"]` and `find-links = ["https://girder.github.io/large_image_wheels"]` so the wheel is preferred over PyPI's source dist
- [x] `uv sync` resolves cleanly and both `from osgeo import gdal` and `from spade import ...` work

## Phase 1 — Shared infrastructure

- [x] `src/gdal_blocks/_common.py` — `gdal.UseExceptions()`, `ogr.UseExceptions()`, `osr.UseExceptions()`, `output_path()`, `srs_from_user()`
- [x] `src/gdal_blocks/_resampling.py` — resampling-method aliases shared by `warp`, `retile`, `translate`, `reduce_mosaic`
- [x] `src/gdal_blocks/_terrain.py` — shared `dem_process()` helper for the seven `gdaldem` blocks
- [x] `src/gdal_blocks/_vector_algebra.py` — shared `run_layer_op()` for the eight `ogr_layer_algebra` blocks
- [x] `tests/conftest.py` — per-test `workdir` fixture plus shared raster / DEM / vector / classified / points fixtures
- [x] `tests/test_infrastructure.py` — 14 smoke tests covering helpers and fixtures

## Phase 2 — Implement blocks

All 47 blocks implemented. Manifests live in `blocks/<name>.yaml`; handlers live in `src/gdal_blocks/<name>.py`; tests live in the category-keyed files under `tests/`.

### 2.1 Raster I/O and format conversion

- [x] `translate` — wraps `gdal.Translate`
- [x] `warp` — wraps `gdal.Warp`
- [x] `merge` — multi-source `gdal.Warp`
- [x] `build_vrt` — wraps `gdal.BuildVRT`
- [x] `add_overviews` — wraps `Dataset.BuildOverviews`
- [x] `tile_index` — footprint index via OGR
- [x] `nearblack` — wraps `gdal.Nearblack`
- [x] `tile` — wraps `osgeo_utils.gdal2tiles`
- [x] `retile` — per-tile `gdal.Translate` windows

### 2.2 Raster ↔ vector conversion

- [x] `rasterize` — wraps `gdal.Rasterize`
- [x] `polygonize` — wraps `gdal.Polygonize`
- [x] `contour` — wraps `gdal.ContourGenerateEx`

### 2.3 Raster analysis

- [x] `calc` — NumPy expression evaluator
- [x] `sieve` — wraps `gdal.SieveFilter`
- [x] `fill_nodata` — wraps `gdal.FillNodata`
- [x] `proximity` — wraps `gdal.ComputeProximity`
- [x] `grid` — wraps `gdal.Grid` (CSV + VRT descriptor)
- [x] `viewshed` — wraps `gdal.ViewshedGenerate`

### 2.4 Clip and mask (convenience)

- [x] `clip_raster_by_vector` — `gdal.Warp` with `cutlineDSName` + `cropToCutline`
- [x] `clip_raster_by_extent` — `gdal.Warp` with `outputBounds`

### 2.5 Terrain (from `gdaldem`)

- [x] `hillshade` — `gdal.DEMProcessing("hillshade")`
- [x] `slope` — `gdal.DEMProcessing("slope")`
- [x] `aspect` — `gdal.DEMProcessing("aspect")`
- [x] `color_relief` — `gdal.DEMProcessing("color-relief")` with color ramp input
- [x] `tri` — `gdal.DEMProcessing("TRI")`
- [x] `tpi` — `gdal.DEMProcessing("TPI")`
- [x] `roughness` — `gdal.DEMProcessing("Roughness")`

### 2.6 Raster information

- [x] `info` — wraps `gdal.Info(format="json")`
- [x] `location_info` — raster query at pixel/georef point
- [x] `compare` — structural + pixel-level diff report

### 2.7 Vector I/O and conversion

- [x] `vector_translate` — wraps `gdal.VectorTranslate`
- [x] `vector_merge` — OGR layer-copy based merge
- [x] `vector_tile_index` — footprint index

### 2.8 Vector layer algebra

- [x] `vector_union` — `Layer.Union`
- [x] `vector_intersection` — `Layer.Intersection`
- [x] `vector_difference` — `Layer.Erase` (A \ B)
- [x] `vector_sym_difference` — `Layer.SymDifference`
- [x] `vector_identity` — `Layer.Identity`
- [x] `vector_clip` — `Layer.Clip`
- [x] `vector_erase` — `Layer.Erase` (alias of `vector_difference`)
- [x] `vector_update` — `Layer.Update`

### 2.9 Vector information

- [x] `vector_info` — wraps `gdal.VectorInfo(format="json")`

### 2.10 CRS and coordinate transforms

- [x] `srs_info` — `osr.SpatialReference` introspection
- [x] `transform_points` — CSV coordinate transform via `osr.CoordinateTransformation`

### 2.11 Map and reduce helpers

- [x] `map_raster_tiles` (`kind: map`) — emits `expansion.yaml` over a raster collection
- [x] `reduce_mosaic` (`kind: reduce`) — fan-in mosaic via `gdal.Warp`
- [x] `reduce_vrt` (`kind: reduce`) — fan-in VRT via `gdal.BuildVRT`

## Phase 3 — Validation

- [x] `uv run pytest tests/` — **97 tests, all passing**
- [x] `spade check` — **47 blocks validated**, zero errors
- [ ] End-to-end smoke pipeline (`examples/smoke.yaml`) — deferred; needs a runnable registry + `spade install`
- [ ] End-to-end map/reduce pipeline (`examples/mosaic.yaml`) — deferred; same dependency

## Phase 4 — Documentation and polish

- [x] Every manifest has a block-level `description` and per-input/output descriptions
- [x] README at `blocks/gdal/README.md` pointing at `SPECIFICATION.md`
- [x] Deferred GDAL utilities noted in `SPECIFICATION.md` (future work: `gdal_edit`, `gdalmanage`, format-specific tools)
