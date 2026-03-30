+++
title = "Geospatial First"
weight = 3
description = "Native support for raster, vector, and tabular geospatial data through GDAL integration."
template = "features/page.html"
+++

Spade was designed from the ground up with geospatial data processing in mind.

## GDAL Block Library

The built-in GDAL block collection provides a comprehensive set of geospatial operations:

- **Rasterize** — Convert vector data to raster format
- **Reproject** — Transform data between coordinate reference systems
- **Clip** — Crop data to a region of interest
- **Merge** — Combine multiple raster datasets
- **Translate** — Convert between raster formats
- **Warp** — Advanced reprojection and resampling

## Data Types

Spade's type system includes first-class geospatial types:

- **RasterFile** — Georeferenced raster data (GeoTIFF, Cloud Optimized GeoTIFF, etc.)
- **VectorFile** — Vector geometry data (GeoJSON, GeoPackage, Shapefile, etc.)
- **TabularFile** — Tabular data with optional spatial columns

## Data Providers

Through OpenDAL integration, Spade can connect to diverse data sources:

- Cloud object storage (S3, GCS, Azure Blob)
- HTTP/HTTPS endpoints
- Local and network filesystems

Data provider blocks handle authentication, pagination, and format conversion automatically.
