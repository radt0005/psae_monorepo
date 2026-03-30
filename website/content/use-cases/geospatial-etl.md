+++
title = "Geospatial ETL"
weight = 2
description = "Extracting, transforming, and loading spatial datasets across formats and coordinate systems."
+++

Spade streamlines geospatial ETL workflows, making it easy to move data between formats, systems, and coordinate reference systems.

## The Challenge

Geospatial ETL involves:

- Ingesting data from diverse sources (APIs, cloud storage, databases)
- Converting between formats (GeoJSON, Shapefile, GeoPackage, GeoTIFF)
- Reprojecting between coordinate reference systems
- Validating and cleaning spatial geometries
- Loading processed data into target systems

## How Spade Helps

Spade's built-in GDAL block library and data provider blocks handle the heavy lifting:

- **Format conversion** — Use GDAL translate and warp blocks for seamless format conversion
- **Reprojection** — Transform between any CRS supported by PROJ
- **Data providers** — Connect to S3, GCS, HTTP endpoints, and more through OpenDAL blocks
- **Type safety** — Spade's type system ensures raster outputs connect to raster inputs, catching wiring errors before execution
- **Reproducibility** — Deterministic execution and caching mean the same pipeline always produces the same results

## Benefits

- **No custom glue code** — Chain existing blocks instead of writing format-specific scripts
- **Parallel execution** — Process multiple datasets concurrently
- **Audit trail** — Full logging of every block execution for compliance and debugging
