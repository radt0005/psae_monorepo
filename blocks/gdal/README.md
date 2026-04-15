# GDAL Blocks

Spade block collection wrapping the GDAL/OGR Python bindings.
47 blocks covering raster I/O, vector I/O, terrain analysis, layer algebra,
and map/reduce helpers. See `SPECIFICATION.md` for the block list and
`IMPLEMENTATION_PLAN.md` for the build history.

## Quick start

```bash
cd blocks/gdal
uv sync
uv run pytest
```

The collection name is `gdal`, so blocks are referenced as `gdal.<name>`
in pipelines (e.g. `gdal.warp`, `gdal.polygonize`). The Python package is
named `gdal_blocks` to avoid shadowing `osgeo.gdal`.

## Environment notes

- Python 3.12 (uv-managed venv)
- GDAL Python bindings from the [large_image_wheels](https://girder.github.io/large_image_wheels/) distribution (pinned to 3.10.0) — the PyPI `gdal` source dist does not compile against the system's bleeding-edge libgdal
- Spade runtime library resolved from the monorepo at `libs/python/` (editable install)
