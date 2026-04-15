+++
title = "data"
description = "Data import blocks for remote storage backends and public datasets. Implemented in Rust."
weight = 2
sort_by = "weight"
insert_anchor_links = "right"
+++

The `data` collection imports data into Spade pipelines. It provides two kinds of blocks:

1. **Generic storage blocks** built on [Apache OpenDAL](https://opendal.apache.org/core/) that read, write, list, and stat objects on any supported storage backend (S3, GCS, Azure Blob, HTTP(S), SFTP, WebDAV, Google Drive, OneDrive, Dropbox, and the local filesystem). The backend is inferred from the URI scheme.
2. **Domain-specific blocks** that know how to download particular public datasets (US Census, USDA, USGS, Natural Earth, OpenStreetMap, NLCD, NHD, PRISM, SSURGO, FIA).

All blocks in this collection declare `network: true` because they fetch data from the internet.

## Generic storage blocks

| Block | Kind | Description |
|-------|------|-------------|
| [`data.read`](/blocks/data/read/) | standard | Fetch a single object from any supported backend |
| [`data.read_collection`](/blocks/data/read_collection/) | standard | Fetch multiple objects matching a prefix or glob |
| [`data.write`](/blocks/data/write/) | standard | Write a single local file to a remote backend |
| [`data.write_collection`](/blocks/data/write_collection/) | standard | Write a collection of local files to a remote prefix |
| [`data.list`](/blocks/data/list/) | standard | List objects under a URI prefix |
| [`data.stat`](/blocks/data/stat/) | standard | Fetch metadata for a single object |

## US Census

| Block | Kind | Description |
|-------|------|-------------|
| [`data.census_acs`](/blocks/data/census_acs/) | standard | Query the US Census ACS Data API |
| [`data.census_tiger`](/blocks/data/census_tiger/) | standard | Download Census TIGER/Line shapefiles |

## Public geospatial datasets

| Block | Kind | Description |
|-------|------|-------------|
| [`data.naturalearth_raster`](/blocks/data/naturalearth_raster/) | standard | Download Natural Earth raster datasets |
| [`data.naturalearth_vector`](/blocks/data/naturalearth_vector/) | standard | Download Natural Earth vector datasets |
| [`data.nhd`](/blocks/data/nhd/) | standard | Download USGS National Hydrography Dataset by HUC |
| [`data.nlcd`](/blocks/data/nlcd/) | standard | Download National Land Cover Database rasters |
| [`data.osm_extract_pbf`](/blocks/data/osm_extract_pbf/) | standard | Download OpenStreetMap extracts in `.osm.pbf` format |
| [`data.osm_extract_shp`](/blocks/data/osm_extract_shp/) | standard | Download OpenStreetMap extracts as shapefiles |
| [`data.prism`](/blocks/data/prism/) | standard | Download PRISM climate rasters |
| [`data.ssurgo`](/blocks/data/ssurgo/) | standard | Download USDA SSURGO soils data |
| [`data.usgs_3dep`](/blocks/data/usgs_3dep/) | standard | Fetch USGS 3DEP elevation tiles overlapping an AOI |
| [`data.fia`](/blocks/data/fia/) | standard | Download USFS Forest Inventory and Analysis (FIA) data |

## Installation

```bash
spade install file:///path/to/blocks/data
```
