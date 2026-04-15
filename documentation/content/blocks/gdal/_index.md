+++
title = "gdal"
description = "Raster and vector operations wrapping the GDAL/OGR command-line utilities. Implemented in Python."
weight = 3
sort_by = "weight"
insert_anchor_links = "right"
+++

The `gdal` collection wraps the [GDAL/OGR](https://gdal.org/) geospatial library using its official Python bindings. Nearly every command-line GDAL utility is available as a block, organized by operation type.

Raster blocks have no prefix; vector (OGR) blocks are prefixed with `vector_` so they group visually in pipeline editors.

## Raster I/O and format conversion

| Block | Wraps | Description |
|-------|-------|-------------|
| [`gdal.translate`](/blocks/gdal/translate/) | `gdal_translate` | Format conversion, subsetting, scaling |
| [`gdal.warp`](/blocks/gdal/warp/) | `gdalwarp` | Reproject, resample, warp |
| [`gdal.merge`](/blocks/gdal/merge/) | `gdal_merge` | Mosaic multiple rasters into one |
| [`gdal.build_vrt`](/blocks/gdal/build_vrt/) | `gdalbuildvrt` | Build a GDAL Virtual Raster |
| [`gdal.add_overviews`](/blocks/gdal/add_overviews/) | `gdaladdo` | Build pyramid overviews |
| [`gdal.tile_index`](/blocks/gdal/tile_index/) | `gdaltindex` | Build a raster tile index |
| [`gdal.nearblack`](/blocks/gdal/nearblack/) | `nearblack` | Clean near-black/white pixels at edges |
| [`gdal.tile`](/blocks/gdal/tile/) | `gdal2tiles` | Generate XYZ/TMS tiles for web mapping |
| [`gdal.retile`](/blocks/gdal/retile/) | `gdal_retile` | Retile a raster into a regular grid |

## Raster ↔ vector conversion

| Block | Wraps | Description |
|-------|-------|-------------|
| [`gdal.rasterize`](/blocks/gdal/rasterize/) | `gdal_rasterize` | Burn vector geometries into a raster |
| [`gdal.polygonize`](/blocks/gdal/polygonize/) | `gdal_polygonize` | Convert raster regions to polygon features |
| [`gdal.contour`](/blocks/gdal/contour/) | `gdal_contour` | Generate contour lines from a raster |

## Raster analysis

| Block | Wraps | Description |
|-------|-------|-------------|
| [`gdal.calc`](/blocks/gdal/calc/) | `gdal_calc` | Raster algebra via NumPy expressions |
| [`gdal.sieve`](/blocks/gdal/sieve/) | `gdal_sieve` | Remove small connected regions |
| [`gdal.fill_nodata`](/blocks/gdal/fill_nodata/) | `gdal_fillnodata` | Interpolate nodata pixels |
| [`gdal.proximity`](/blocks/gdal/proximity/) | `gdal_proximity` | Distance-to-features raster |
| [`gdal.grid`](/blocks/gdal/grid/) | `gdal_grid` | Interpolate scattered points onto a grid |
| [`gdal.viewshed`](/blocks/gdal/viewshed/) | `gdal_viewshed` | Viewshed analysis |

## Clip and mask

| Block | Wraps | Description |
|-------|-------|-------------|
| [`gdal.clip_raster_by_vector`](/blocks/gdal/clip_raster_by_vector/) | `gdalwarp -cutline` | Clip a raster to a vector mask |
| [`gdal.clip_raster_by_extent`](/blocks/gdal/clip_raster_by_extent/) | `gdalwarp` (bbox) | Clip a raster to a bounding box |

## Terrain analysis (from `gdaldem`)

| Block | Description |
|-------|-------------|
| [`gdal.hillshade`](/blocks/gdal/hillshade/) | Hillshade from a DEM |
| [`gdal.slope`](/blocks/gdal/slope/) | Slope from a DEM |
| [`gdal.aspect`](/blocks/gdal/aspect/) | Aspect from a DEM |
| [`gdal.color_relief`](/blocks/gdal/color_relief/) | Color relief from a DEM |
| [`gdal.tri`](/blocks/gdal/tri/) | Terrain Ruggedness Index |
| [`gdal.tpi`](/blocks/gdal/tpi/) | Topographic Position Index |
| [`gdal.roughness`](/blocks/gdal/roughness/) | Terrain roughness |

## Raster information

| Block | Wraps | Description |
|-------|-------|-------------|
| [`gdal.info`](/blocks/gdal/info/) | `gdalinfo` | Raster metadata report |
| [`gdal.location_info`](/blocks/gdal/location_info/) | `gdallocationinfo` | Query raster band values at a point |
| [`gdal.compare`](/blocks/gdal/compare/) | `gdalcompare` | Compare two rasters |

## Vector I/O and conversion

| Block | Wraps | Description |
|-------|-------|-------------|
| [`gdal.vector_translate`](/blocks/gdal/vector_translate/) | `ogr2ogr` | Format conversion, reprojection, filtering |
| [`gdal.vector_merge`](/blocks/gdal/vector_merge/) | `ogrmerge` | Merge vector files |
| [`gdal.vector_tile_index`](/blocks/gdal/vector_tile_index/) | `ogrtindex` | Build a vector tile index |

## Vector analysis (from `ogr_layer_algebra`)

| Block | Description |
|-------|-------------|
| [`gdal.vector_union`](/blocks/gdal/vector_union/) | Union of two layers |
| [`gdal.vector_intersection`](/blocks/gdal/vector_intersection/) | Intersection of two layers |
| [`gdal.vector_difference`](/blocks/gdal/vector_difference/) | Difference (A minus B) |
| [`gdal.vector_sym_difference`](/blocks/gdal/vector_sym_difference/) | Symmetric difference |
| [`gdal.vector_identity`](/blocks/gdal/vector_identity/) | Identity overlay |
| [`gdal.vector_clip`](/blocks/gdal/vector_clip/) | Clip layer A by layer B |
| [`gdal.vector_erase`](/blocks/gdal/vector_erase/) | Erase features of A by B |
| [`gdal.vector_update`](/blocks/gdal/vector_update/) | Update layer A with B |

## Vector information

| Block | Wraps | Description |
|-------|-------|-------------|
| [`gdal.vector_info`](/blocks/gdal/vector_info/) | `ogrinfo` | Vector metadata report |

## CRS and coordinate transforms

| Block | Wraps | Description |
|-------|-------|-------------|
| [`gdal.srs_info`](/blocks/gdal/srs_info/) | `gdalsrsinfo` | CRS metadata report |
| [`gdal.transform_points`](/blocks/gdal/transform_points/) | `gdaltransform` | Transform coordinates between CRSs |

## Map and reduce helpers

| Block | Kind | Description |
|-------|------|-------------|
| [`gdal.map_raster_tiles`](/blocks/gdal/map_raster_tiles/) | map | Enumerate a raster collection for per-tile processing |
| [`gdal.reduce_mosaic`](/blocks/gdal/reduce_mosaic/) | reduce | Mosaic a collection of rasters back into one |
| [`gdal.reduce_vrt`](/blocks/gdal/reduce_vrt/) | reduce | Build a VRT from a collection of rasters |

## Installation

```bash
spade install file:///path/to/blocks/gdal
```

## See also

- [GDAL command-line tools documentation](https://gdal.org/programs/)
- [Core Concepts: Map/Reduce](/concepts/map-reduce/) for using `map_raster_tiles` and `reduce_mosaic`/`reduce_vrt`
