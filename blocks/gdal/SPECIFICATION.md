# GDAL Blocks

Please see the specifications for this system in `../../spec` and the skill at `../../skills/spade`.

This collection of blocks is based on the Geospatial Data Analytics Library (GDAL). It should use the official Python GDAL bindings to cover the entire API surface of GDAL, both for raster and vector data.

All block IDs are `gdal.<name>`. OGR (vector) operations use a `vector_` prefix so related blocks group visually in the flowchart UI; raster operations are unprefixed.

## Blocks

### Raster I/O and format conversion

| Block | Wraps | Description |
| --- | --- | --- |
| `translate` | `gdal_translate` | Format conversion, subsetting, scaling |
| `warp` | `gdalwarp` | Reproject, resample, warp |
| `merge` | `gdal_merge` | Mosaic multiple rasters into one |
| `build_vrt` | `gdalbuildvrt` | Build a GDAL Virtual Raster |
| `add_overviews` | `gdaladdo` | Build pyramid overviews |
| `tile_index` | `gdaltindex` | Build a raster tile index |
| `nearblack` | `nearblack` | Clean near-black/white pixels at edges |
| `tile` | `gdal2tiles` | Generate XYZ/TMS tiles for web mapping |
| `retile` | `gdal_retile` | Retile a raster into a regular grid |

### Raster ↔ vector conversion

| Block | Wraps | Description |
| --- | --- | --- |
| `rasterize` | `gdal_rasterize` | Burn vector geometries into a raster |
| `polygonize` | `gdal_polygonize` | Convert raster regions to polygon features |
| `contour` | `gdal_contour` | Generate contour lines or polygons |

### Raster analysis

| Block | Wraps | Description |
| --- | --- | --- |
| `calc` | `gdal_calc` | Raster algebra |
| `sieve` | `gdal_sieve` | Remove small connected regions |
| `fill_nodata` | `gdal_fillnodata` | Interpolate nodata pixels |
| `proximity` | `gdal_proximity` | Distance-to-features raster |
| `grid` | `gdal_grid` | Interpolate scattered points onto a grid |
| `viewshed` | `gdal_viewshed` | Viewshed analysis |

### Clip and mask (convenience)

| Block | Wraps | Description |
| --- | --- | --- |
| `clip_raster_by_vector` | `gdalwarp` (cutline) | Clip a raster to a vector mask |
| `clip_raster_by_extent` | `gdalwarp` (bbox) | Clip a raster to a bounding box |

### Terrain (from `gdaldem`)

| Block | Description |
| --- | --- |
| `hillshade` | Hillshade from a DEM |
| `slope` | Slope from a DEM |
| `aspect` | Aspect from a DEM |
| `color_relief` | Color relief from a DEM |
| `tri` | Terrain ruggedness index |
| `tpi` | Topographic position index |
| `roughness` | Terrain roughness |

### Raster information

| Block | Wraps | Description |
| --- | --- | --- |
| `info` | `gdalinfo` | Raster metadata report |
| `location_info` | `gdallocationinfo` | Query raster value(s) at a point |
| `compare` | `gdalcompare` | Compare two rasters |

### Vector I/O and conversion

| Block | Wraps | Description |
| --- | --- | --- |
| `vector_translate` | `ogr2ogr` | Format conversion, reprojection, filtering |
| `vector_merge` | `ogrmerge` | Merge vector files |
| `vector_tile_index` | `ogrtindex` | Build a vector tile index |

### Vector analysis (from `ogr_layer_algebra`)

| Block | Description |
| --- | --- |
| `vector_union` | Union of two layers |
| `vector_intersection` | Intersection of two layers |
| `vector_difference` | Difference (A minus B) |
| `vector_sym_difference` | Symmetric difference |
| `vector_identity` | Identity overlay |
| `vector_clip` | Clip layer A by layer B |
| `vector_erase` | Erase features of A by B |
| `vector_update` | Update layer A with B |

### Vector information

| Block | Wraps | Description |
| --- | --- | --- |
| `vector_info` | `ogrinfo` | Vector metadata report |

### CRS and coordinate transforms

| Block | Wraps | Description |
| --- | --- | --- |
| `srs_info` | `gdalsrsinfo` | CRS metadata report |
| `transform_points` | `gdaltransform` | Transform coordinates between CRSs |

### Map and reduce helpers

| Block | Kind | Description |
| --- | --- | --- |
| `map_raster_tiles` | `map` | Enumerate a raster collection for per-tile processing |
| `reduce_mosaic` | `reduce` | Mosaic a collection of rasters back into one |
| `reduce_vrt` | `reduce` | Build a VRT from a collection of rasters |
