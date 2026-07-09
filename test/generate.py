#!/usr/bin/env python3
"""Generate test fixtures and pipeline YAML files for spade integration tests.

This script:
  1. Creates small test data files in ./fixtures/
  2. Generates pipeline YAML files in ./pipelines/ with proper UUIDv7 block IDs

The pipelines cover all implemented blocks in the base, gdal, and data
collections, organized into local tests (no network) and network tests
(public dataset blocks).
"""

import json
import os
import struct
import time
from pathlib import Path

import numpy as np
import rasterio
import yaml
from rasterio.crs import CRS
from rasterio.transform import from_bounds

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------

ROOT = Path(__file__).resolve().parent
FIXTURES = ROOT / "fixtures"
PIPELINES = ROOT / "pipelines"

# ---------------------------------------------------------------------------
# UUIDv7 generator
# ---------------------------------------------------------------------------

_uuid_counter = 0


def uuid7() -> str:
    """Return a fresh UUIDv7 string.  Uses millisecond timestamp + random bits."""
    global _uuid_counter
    _uuid_counter += 1
    ts_ms = int(time.time() * 1000)
    rand = os.urandom(10)
    # inject counter into random bytes to guarantee uniqueness within a run
    rand = (
        rand[:2]
        + _uuid_counter.to_bytes(2, "big")
        + rand[4:]
    )
    b = ts_ms.to_bytes(6, "big")                               # 48-bit timestamp
    b += bytes([(rand[0] & 0x0F) | 0x70, rand[1]])             # version 7
    b += bytes([(rand[2] & 0x3F) | 0x80])                      # variant 10
    b += rand[3:10]
    h = b.hex()
    return f"{h[:8]}-{h[8:12]}-{h[12:16]}-{h[16:20]}-{h[20:]}"


# ---------------------------------------------------------------------------
# Default scalar args for every block (the validator requires all scalar
# inputs to be present).  Values match the documented defaults.
# ---------------------------------------------------------------------------

BLOCK_DEFAULTS: dict[str, dict[str, object]] = {
    # ---- base ----
    "base.aggregate":          {"aggregations": "[]", "group_by": ""},
    "base.csv_to_parquet":     {"delimiter": ",", "has_header": True},
    "base.filter_rows":        {"expression": ""},
    "base.group_by":           {"group_columns": "", "aggregations": "[]"},
    "base.map_list":           {"values": "[]"},
    "base.map_range":          {"start": 0, "end": 1, "step": 1},
    "base.parquet_to_csv":     {"delimiter": ",", "include_header": True},
    "base.reduce_join":        {"on": "", "how": "inner"},
    "base.reduce_stack":       {"strict": False},
    "base.select_columns":     {"columns": "", "mode": "keep"},
    # ---- data ----
    "data.read":               {"uri": "", "format": ""},
    "data.read_collection":    {"uri": "", "format": "", "max_items": 0},
    "data.write":              {"uri": "", "overwrite": False},
    "data.write_collection":   {"uri": "", "overwrite": False},
    "data.list":               {"uri": "", "recursive": False},
    "data.stat":               {"uri": ""},
    "data.census_acs":         {"year": 2022, "dataset": "acs5", "table": "B01003",
                                "geography": "state:*", "variables": ""},
    "data.census_tiger":       {"year": 2022, "layer": "states", "state": ""},
    "data.fia":                {"state": "all", "tables": ""},
    "data.nlcd":               {"year": 2021, "product": "land_cover", "region": "CONUS"},
    "data.prism":              {"variable": "tmean", "start": "2023-01-01",
                                "end": "2023-01-31", "resolution": "4km", "cadence": "monthly"},
    "data.ssurgo":             {"area": "", "state": ""},
    "data.usgs_3dep":          {"resolution": "30m", "product": "DEM"},
    "data.nhd":                {"huc": "0109", "resolution": "high"},
    "data.naturalearth_raster": {"scale": "50m", "theme": "HYP_HR_SR_OB_DR"},
    "data.naturalearth_vector": {"scale": "110m", "category": "cultural", "theme": "admin_0_countries"},
    "data.osm_extract_pbf":    {"region": "north-america/us/rhode-island"},
    "data.osm_extract_shp":    {"region": "north-america/us/rhode-island"},
    # ---- gdal ----
    "gdal.translate":          {"output_format": "GTiff", "output_type": "", "width": 0,
                                "height": 0, "scale_min": 0, "scale_max": 0},
    "gdal.warp":               {"target_crs": "", "resolution": 0, "resampling": "nearest",
                                "output_format": "GTiff"},
    "gdal.merge":              {"resampling": "nearest", "output_format": "GTiff"},
    "gdal.build_vrt":          {"resolution": "highest", "separate": False},
    "gdal.add_overviews":      {"levels": "2 4 8 16", "resampling": "average"},
    "gdal.tile_index":         {"location_field": "location"},
    "gdal.nearblack":          {"near": 15, "white": False, "set_alpha": False},
    "gdal.tile":               {"zoom": "0-5", "profile": "mercator", "resampling": "average"},
    "gdal.retile":             {"tile_width": 256, "tile_height": 256},
    "gdal.rasterize":          {"burn_value": 1, "attribute": "", "all_touched": False},
    "gdal.polygonize":         {"band": 1, "field_name": "DN", "connectedness": 4},
    "gdal.contour":            {"band": 1, "interval": 1.0, "base": 0, "field_name": "elev"},
    "gdal.calc":               {"expression": "A", "output_type": "Float32", "nodata": 0},
    "gdal.sieve":              {"threshold": 10, "connectedness": 4},
    "gdal.fill_nodata":        {"max_distance": 100, "smoothing_iterations": 0},
    "gdal.proximity":          {"target_values": "", "distance_units": "pixel", "max_distance": 0},
    "gdal.grid":               {"algorithm": "invdist:power=2.0", "width": 64, "height": 64,
                                "z_field": "z", "x_field": "x", "y_field": "y"},
    "gdal.viewshed":           {"observer_x": 0, "observer_y": 0, "observer_height": 1.6,
                                "target_height": 0, "max_distance": 0},
    "gdal.clip_raster_by_vector": {"crop_to_cutline": True, "all_touched": False},
    "gdal.clip_raster_by_extent": {"xmin": 0, "ymin": 0, "xmax": 0, "ymax": 0},
    "gdal.hillshade":          {"azimuth": 315, "altitude": 45, "z_factor": 1, "scale": 1,
                                "multidirectional": False},
    "gdal.slope":              {"scale": 1, "slope_format": "degree"},
    "gdal.aspect":             {"zero_for_flat": False, "trigonometric": False},
    "gdal.color_relief":       {},
    "gdal.tri":                {"algorithm": "Wilson"},
    "gdal.tpi":                {},
    "gdal.roughness":          {},
    "gdal.info":               {"compute_stats": False},
    "gdal.location_info":      {"x": 0, "y": 0, "coord_system": "georef"},
    "gdal.compare":            {},
    "gdal.vector_translate":   {"output_format": "GeoJSON", "target_crs": "", "where": "", "sql": ""},
    "gdal.vector_merge":       {"output_format": "GeoJSON"},
    "gdal.vector_tile_index":  {"location_field": "location"},
    "gdal.vector_union":       {},
    "gdal.vector_intersection": {},
    "gdal.vector_difference":  {},
    "gdal.vector_sym_difference": {},
    "gdal.vector_identity":    {},
    "gdal.vector_clip":        {},
    "gdal.vector_erase":       {},
    "gdal.vector_update":      {},
    "gdal.vector_info":        {},
    "gdal.srs_info":           {"crs": "EPSG:4326"},
    "gdal.transform_points":   {"source_crs": "", "target_crs": "", "x_field": "x", "y_field": "y"},
    "gdal.map_raster_tiles":   {},
    "gdal.reduce_mosaic":      {"resampling": "nearest"},
    "gdal.reduce_vrt":         {"resolution": "highest"},
    # ---- stats ----
    "stats.summary":           {"columns": ""},
    "stats.correlation":       {"method": "pearson", "columns": ""},
    "stats.frequency":         {"column": "", "by": ""},
    "stats.t_test":            {"value_column": "", "group_column": "", "mu": 0,
                                "alternative": "two.sided", "paired": False,
                                "conf_level": 0.95},
    "stats.anova":             {"value_column": "", "group_column": ""},
    "stats.chisq_test":        {"column": "", "by": "", "correct": True},
    # ---- fiadb ----
    "fiadb.fullreport":        {"wc": "", "snum": "", "sdenom": "", "rselected": "",
                                "cselected": "", "pselected": "", "rtime": "", "ctime": "",
                                "ptime": "", "strFilter": "", "FIAorRPA": "", "estOnly": ""},
    "fiadb.parameters":        {"name": ""},
    "fiadb.area":              {"state": "", "year": "", "group_by": "", "land_basis": "", "units": ""},
    "fiadb.volume":            {"state": "", "year": "", "group_by": "", "land_basis": "", "units": ""},
    "fiadb.biomass":           {"state": "", "year": "", "group_by": "", "land_basis": "", "units": ""},
    "fiadb.carbon":            {"state": "", "year": "", "group_by": "", "land_basis": "", "units": ""},
}


# ---------------------------------------------------------------------------
# Pipeline builder helpers
# ---------------------------------------------------------------------------


def pipeline(name: str, description: str, blocks: list[dict]) -> dict:
    return {
        "id": uuid7(),
        "name": name,
        "version": "1.0",
        "description": description,
        "blocks": blocks,
    }


def block(name: str, inputs=None, args=None) -> dict:
    """Return a pipeline block dict with a fresh invocation ID.

    Merges caller-supplied *args* on top of BLOCK_DEFAULTS so that every
    scalar input the validator expects is present.
    """
    merged = dict(BLOCK_DEFAULTS.get(name, {}))
    if args:
        merged.update(args)
    b = {"id": uuid7(), "name": name}
    b["inputs"] = inputs or []
    b["args"] = merged
    return b


def bare(blk: dict) -> str:
    """Return the invocation ID string for a bare input reference."""
    return blk["id"]


def explicit(blk: dict, output: str, as_input: str | None = None) -> dict:
    """Return an explicit input reference.

    If *as_input* is given, target that named downstream input — used when
    the receiving block has multiple same-typed inputs.
    """
    ref = {"block": blk["id"], "output": output}
    if as_input is not None:
        ref["as"] = as_input
    return ref


def _quoted_str_representer(dumper, data):
    """Force quotes on strings that could be parsed as numbers by YAML 1.2.

    Go's yaml.v3 follows YAML 1.2 and reads bare tokens like "0109" as the
    integer 109 (leading zeros stripped); PyYAML writes them unquoted.
    Quote such strings explicitly so the round-trip preserves the type.
    """
    if data and (data[0].isdigit() or data[0] in "+-.") and data.replace(".", "", 1).replace("e", "", 1).replace("-", "", 1).replace("+", "", 1).isdigit():
        return dumper.represent_scalar("tag:yaml.org,2002:str", data, style='"')
    return dumper.represent_scalar("tag:yaml.org,2002:str", data)


yaml.SafeDumper.add_representer(str, _quoted_str_representer)


def save(p: dict):
    """Write a pipeline dict to pipelines/<name>.yaml."""
    path = PIPELINES / f"{p['name']}.yaml"
    with open(path, "w") as f:
        yaml.safe_dump(p, f, default_flow_style=False, sort_keys=False)
    return path


# ---------------------------------------------------------------------------
# Fixture generation
# ---------------------------------------------------------------------------


def fixture_path(name: str) -> str:
    """Return the absolute path to a fixture file (as string for YAML args).

    When *name* is empty the path ends with a trailing slash so that glob
    patterns can be concatenated directly (e.g. ``fixture_path("") + "*.csv"``).
    """
    if name == "":
        return str(FIXTURES) + "/"
    return str(FIXTURES / name)


def create_fixtures():
    """Create all test data fixtures."""
    FIXTURES.mkdir(exist_ok=True)

    # --- CSV files ---
    _csv(
        "test_data.csv",
        ["id", "name", "value", "category", "state"],
        [
            ["1", "alpha", "10.5", "A", "NY"],
            ["2", "beta", "20.3", "B", "CA"],
            ["3", "gamma", "15.7", "A", "NY"],
            ["4", "delta", "30.1", "B", "MI"],
            ["5", "epsilon", "25.9", "A", "CA"],
            ["6", "zeta", "12.4", "C", "MI"],
            ["7", "eta", "18.6", "A", "NY"],
            ["8", "theta", "22.0", "B", "CA"],
        ],
    )

    _csv(
        "test_data2.csv",
        ["id", "name", "value", "category", "state"],
        [
            ["1", "alpha2", "100.0", "X", "TX"],
            ["2", "beta2", "200.0", "Y", "FL"],
            ["3", "gamma2", "150.0", "X", "TX"],
            ["4", "delta2", "300.0", "Y", "FL"],
        ],
    )

    _csv(
        "test_data3.csv",
        ["id", "name", "value", "category", "state"],
        [
            ["5", "eps3", "50.0", "Z", "WA"],
            ["6", "zeta3", "60.0", "Z", "OR"],
        ],
    )

    # Stats fixture: two numeric columns (x, y) for descriptive/correlation
    # blocks, a two-level grouping column (group) for two-sample t-tests and
    # ANOVA, and a second categorical column (region) for frequency and
    # chi-squared independence tests.
    _csv(
        "stats_data.csv",
        ["group", "region", "x", "y"],
        [
            ["A", "east", "1.0", "10.2"],
            ["A", "west", "2.1", "11.8"],
            ["A", "east", "1.7", "10.9"],
            ["A", "west", "2.4", "12.5"],
            ["B", "east", "3.2", "9.4"],
            ["B", "west", "4.1", "8.9"],
            ["B", "east", "3.8", "9.1"],
            ["B", "west", "4.6", "8.2"],
            ["A", "east", "1.3", "10.5"],
            ["B", "west", "3.9", "8.7"],
        ],
    )

    # Join-test fixtures: each shares only the 'id' column so that a
    # chained join produces distinct non-key columns without name clashes.
    _csv(
        "join_a.csv",
        ["id", "name"],
        [["1", "alpha"], ["2", "beta"], ["3", "gamma"], ["4", "delta"]],
    )
    _csv(
        "join_b.csv",
        ["id", "score"],
        [["1", "95"], ["2", "82"], ["3", "76"], ["4", "88"]],
    )
    _csv(
        "join_c.csv",
        ["id", "grade"],
        [["1", "A"], ["2", "B"], ["3", "C"], ["4", "B"]],
    )

    _csv(
        "test_points.csv",
        ["x", "y", "z"],
        [
            ["-122.4", "37.8", "100"],
            ["-122.3", "37.7", "200"],
            ["-122.2", "37.9", "150"],
            ["-122.5", "37.6", "300"],
            ["-122.1", "37.75", "250"],
            ["-122.35", "37.85", "175"],
            ["-122.45", "37.65", "125"],
            ["-122.25", "37.95", "225"],
        ],
    )

    # --- GeoJSON files ---
    _geojson_polygons("test_vectors.geojson", [
        [[-122.5, 37.5], [-122.0, 37.5], [-122.0, 38.0], [-122.5, 38.0], [-122.5, 37.5]],
        [[-122.3, 37.6], [-121.8, 37.6], [-121.8, 38.1], [-122.3, 38.1], [-122.3, 37.6]],
    ])

    _geojson_polygons("test_vectors2.geojson", [
        [[-122.4, 37.7], [-122.1, 37.7], [-122.1, 37.9], [-122.4, 37.9], [-122.4, 37.7]],
        [[-122.6, 37.4], [-122.2, 37.4], [-122.2, 37.8], [-122.6, 37.8], [-122.6, 37.4]],
    ])

    # --- Raster files ---
    _create_raster(
        "test_raster.tif",
        data=np.random.default_rng(42).random((64, 64), dtype=np.float32) * 255,
        bounds=(-122.5, 37.5, -122.0, 38.0),
    )

    _create_raster(
        "test_raster2.tif",
        data=np.random.default_rng(99).random((64, 64), dtype=np.float32) * 255,
        bounds=(-122.5, 37.5, -122.0, 38.0),
    )

    # A small DEM (elevation-like values)
    rng = np.random.default_rng(7)
    dem = (rng.random((64, 64), dtype=np.float32) * 500 + 100)  # 100-600 m
    _create_raster("test_dem.tif", data=dem, bounds=(-122.5, 37.5, -122.0, 38.0))

    # A small integer raster for sieve / polygonize
    cat = rng.integers(1, 5, size=(64, 64), dtype=np.int32)
    _create_raster(
        "test_categorical.tif",
        data=cat.astype(np.float32),
        bounds=(-122.5, 37.5, -122.0, 38.0),
        dtype="int32",
    )

    # A raster with some nodata pixels for fill_nodata
    nodata_raster = dem.copy()
    nodata_raster[10:20, 10:20] = -9999.0
    _create_raster(
        "test_nodata_raster.tif",
        data=nodata_raster,
        bounds=(-122.5, 37.5, -122.0, 38.0),
        nodata=-9999.0,
    )

    # Binary raster for proximity (some 1s, mostly 0s)
    binary = np.zeros((64, 64), dtype=np.float32)
    binary[30:35, 30:35] = 1.0
    _create_raster("test_binary.tif", data=binary, bounds=(-122.5, 37.5, -122.0, 38.0))

    # --- Color ramp file ---
    with open(FIXTURES / "color_ramp.txt", "w") as f:
        f.write("100 0 128 0\n")
        f.write("200 0 255 0\n")
        f.write("300 255 255 0\n")
        f.write("400 255 128 0\n")
        f.write("500 255 0 0\n")
        f.write("600 128 0 0\n")

    print(f"Fixtures created in {FIXTURES}")


def _csv(name: str, headers: list[str], rows: list[list[str]]):
    path = FIXTURES / name
    with open(path, "w") as f:
        f.write(",".join(headers) + "\n")
        for row in rows:
            f.write(",".join(row) + "\n")


def _geojson_polygons(name: str, rings: list[list[list[float]]]):
    features = []
    for i, ring in enumerate(rings):
        features.append({
            "type": "Feature",
            "properties": {"id": i + 1, "name": f"poly_{i + 1}"},
            "geometry": {"type": "Polygon", "coordinates": [ring]},
        })
    fc = {"type": "FeatureCollection", "features": features}
    with open(FIXTURES / name, "w") as f:
        json.dump(fc, f)


def _create_raster(
    name: str,
    data: np.ndarray,
    bounds: tuple[float, float, float, float],
    dtype: str = "float32",
    nodata: float | None = None,
):
    h, w = data.shape
    transform = from_bounds(*bounds, w, h)
    profile = {
        "driver": "GTiff",
        "height": h,
        "width": w,
        "count": 1,
        "dtype": dtype,
        "crs": CRS.from_epsg(4326),
        "transform": transform,
    }
    if nodata is not None:
        profile["nodata"] = nodata
    path = FIXTURES / name
    with rasterio.open(path, "w", **profile) as dst:
        if dtype == "int32":
            dst.write(data.astype(np.int32), 1)
        else:
            dst.write(data.astype(np.float32), 1)


# ---------------------------------------------------------------------------
# Pipeline definitions
# ---------------------------------------------------------------------------


def generate_pipelines():
    """Generate all pipeline YAML files."""
    PIPELINES.mkdir(exist_ok=True)
    # Clear old pipelines
    for f in PIPELINES.glob("*.yaml"):
        f.unlink()

    generated = []
    generated += _base_pipelines()
    generated += _gdal_pipelines()
    generated += _data_pipelines()
    generated += _stats_pipelines()
    generated += _fiadb_pipelines()

    print(f"Generated {len(generated)} pipelines in {PIPELINES}")
    return generated


# ---- Base collection pipelines ----

def _base_pipelines() -> list[str]:
    paths = []
    fp = fixture_path

    # 1. csv_to_parquet
    read = block("data.read", args={"uri": fp("test_data.csv"), "format": "CSV"})
    conv = block("base.csv_to_parquet", inputs=[bare(read)])
    paths.append(save(pipeline(
        "base_csv_to_parquet",
        "Read CSV and convert to Parquet",
        [read, conv],
    )))

    # 2. parquet_to_csv (roundtrip)
    read = block("data.read", args={"uri": fp("test_data.csv"), "format": "CSV"})
    to_pq = block("base.csv_to_parquet", inputs=[bare(read)])
    to_csv = block("base.parquet_to_csv", inputs=[bare(to_pq)])
    paths.append(save(pipeline(
        "base_parquet_to_csv",
        "CSV -> Parquet -> CSV roundtrip",
        [read, to_pq, to_csv],
    )))

    # 3. filter_rows
    read = block("data.read", args={"uri": fp("test_data.csv"), "format": "CSV"})
    to_pq = block("base.csv_to_parquet", inputs=[bare(read)])
    filt = block("base.filter_rows", inputs=[bare(to_pq)], args={"expression": "value > 15"})
    paths.append(save(pipeline(
        "base_filter_rows",
        "Filter rows where value > 15",
        [read, to_pq, filt],
    )))

    # 4. select_columns
    read = block("data.read", args={"uri": fp("test_data.csv"), "format": "CSV"})
    to_pq = block("base.csv_to_parquet", inputs=[bare(read)])
    sel = block("base.select_columns", inputs=[bare(to_pq)], args={"columns": "id,name,value", "mode": "keep"})
    paths.append(save(pipeline(
        "base_select_columns",
        "Select id, name, value columns",
        [read, to_pq, sel],
    )))

    # 5. aggregate
    read = block("data.read", args={"uri": fp("test_data.csv"), "format": "CSV"})
    to_pq = block("base.csv_to_parquet", inputs=[bare(read)])
    agg = block("base.aggregate", inputs=[bare(to_pq)], args={
        "aggregations": json.dumps([
            {"column": "value", "function": "mean"},
            {"column": "value", "function": "sum"},
            {"column": "id", "function": "count"},
        ]),
        "group_by": "",
    })
    paths.append(save(pipeline(
        "base_aggregate",
        "Compute aggregate statistics on value column",
        [read, to_pq, agg],
    )))

    # 6. group_by
    read = block("data.read", args={"uri": fp("test_data.csv"), "format": "CSV"})
    to_pq = block("base.csv_to_parquet", inputs=[bare(read)])
    grp = block("base.group_by", inputs=[bare(to_pq)], args={
        "group_columns": "state",
        "aggregations": json.dumps([
            {"column": "value", "function": "mean"},
            {"column": "id", "function": "count"},
        ]),
    })
    paths.append(save(pipeline(
        "base_group_by",
        "Group by state and aggregate",
        [read, to_pq, grp],
    )))

    # 7. filter -> select -> aggregate chain
    read = block("data.read", args={"uri": fp("test_data.csv"), "format": "CSV"})
    to_pq = block("base.csv_to_parquet", inputs=[bare(read)])
    filt = block("base.filter_rows", inputs=[bare(to_pq)], args={"expression": "category = 'A'"})
    sel = block("base.select_columns", inputs=[bare(filt)], args={"columns": "id,value,state", "mode": "keep"})
    agg = block("base.aggregate", inputs=[bare(sel)], args={
        "aggregations": json.dumps([{"column": "value", "function": "mean"}]),
        "group_by": "state",
    })
    paths.append(save(pipeline(
        "base_filter_select_aggregate",
        "Filter category=A, select columns, aggregate by state",
        [read, to_pq, filt, sel, agg],
    )))

    # 8. map_files -> csv_to_parquet -> reduce_collection
    read_coll = block("data.read_collection", args={
        "uri": fixture_path("") + "test_data*.csv",
        "format": "CSV",
        "max_items": 10,
    })
    map_f = block("base.map_files", inputs=[bare(read_coll)])
    conv = block("base.csv_to_parquet", inputs=[bare(map_f)])
    red = block("base.reduce_collection", inputs=[bare(conv)])
    paths.append(save(pipeline(
        "base_map_files_reduce_collection",
        "Map over CSV collection, convert each to Parquet, reduce to collection",
        [read_coll, map_f, conv, red],
    )))

    # 9. map_files -> csv_to_parquet -> reduce_stack
    read_coll = block("data.read_collection", args={
        "uri": fixture_path("") + "test_data*.csv",
        "format": "CSV",
        "max_items": 10,
    })
    map_f = block("base.map_files", inputs=[bare(read_coll)])
    conv = block("base.csv_to_parquet", inputs=[bare(map_f)])
    red = block("base.reduce_stack", inputs=[bare(conv)])
    paths.append(save(pipeline(
        "base_map_files_reduce_stack",
        "Map over CSV collection, convert each to Parquet, stack rows",
        [read_coll, map_f, conv, red],
    )))

    # 10. map_files -> csv_to_parquet -> reduce_join
    # Uses dedicated join_*.csv fixtures that share only the 'id' column.
    read_coll = block("data.read_collection", args={
        "uri": fixture_path("") + "join_*.csv",
        "format": "CSV",
        "max_items": 10,
    })
    map_f = block("base.map_files", inputs=[bare(read_coll)])
    conv = block("base.csv_to_parquet", inputs=[bare(map_f)])
    red = block("base.reduce_join", inputs=[bare(conv)], args={"on": "id", "how": "inner"})
    paths.append(save(pipeline(
        "base_map_files_reduce_join",
        "Map over join_*.csv, convert to Parquet, inner-join on id",
        [read_coll, map_f, conv, red],
    )))

    # 11. map_list -> reduce_collection
    ml = block("base.map_list", args={"values": '["NY","CA","MI"]'})
    red = block("base.reduce_collection", inputs=[bare(ml)])
    paths.append(save(pipeline(
        "base_map_list",
        "Fan out over literal list of state codes and reduce",
        [ml, red],
    )))

    # 12. map_range -> reduce_collection
    mr = block("base.map_range", args={"start": 1, "end": 5, "step": 1})
    red = block("base.reduce_collection", inputs=[bare(mr)])
    paths.append(save(pipeline(
        "base_map_range",
        "Fan out over numeric range 1..5 and reduce",
        [mr, red],
    )))

    # 13. Nested map/reduce (depth 2):
    #     map_files -> map_files -> csv_to_parquet -> reduce -> reduce
    # Each outer item is a single CSV, so every inner instance expands to
    # exactly one item — the counts are trivial but the full nested
    # machinery (per-instance expansions, inner reduce per outer index,
    # outer reduce once) is exercised end-to-end.
    read_coll = block("data.read_collection", args={
        "uri": fixture_path("") + "test_data*.csv",
        "format": "CSV",
        "max_items": 10,
    })
    outer_map = block("base.map_files", inputs=[bare(read_coll)])
    inner_map = block("base.map_files", inputs=[bare(outer_map)])
    conv = block("base.csv_to_parquet", inputs=[bare(inner_map)])
    inner_red = block("base.reduce_collection", inputs=[bare(conv)])
    outer_red = block("base.reduce_collection", inputs=[bare(inner_red)])
    paths.append(save(pipeline(
        "base_map_nested",
        "Nested map/reduce: fan out over CSVs, re-map each, convert, reduce twice",
        [read_coll, outer_map, inner_map, conv, inner_red, outer_red],
    )))

    return paths


# ---- GDAL collection pipelines ----

def _gdal_pipelines() -> list[str]:
    paths = []
    fp = fixture_path

    # --- Raster I/O and Format Conversion ---

    # 1. translate
    read = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    xlat = block("gdal.translate", inputs=[bare(read)], args={"output_format": "GTiff", "output_type": "Byte"})
    paths.append(save(pipeline("gdal_translate", "Translate raster to Byte type", [read, xlat])))

    # 2. warp
    read = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    wrp = block("gdal.warp", inputs=[bare(read)], args={"target_crs": "EPSG:32610", "resampling": "bilinear"})
    paths.append(save(pipeline("gdal_warp", "Reproject raster to UTM zone 10N", [read, wrp])))

    # 3. merge
    read_coll = block("data.read_collection", args={
        "uri": fp("") + "test_raster*.tif",
        "format": "GeoTIFF",
        "max_items": 10,
    })
    mrg = block("gdal.merge", inputs=[bare(read_coll)], args={"resampling": "nearest"})
    paths.append(save(pipeline("gdal_merge", "Mosaic raster collection", [read_coll, mrg])))

    # 4. build_vrt
    read_coll = block("data.read_collection", args={
        "uri": fp("") + "test_raster*.tif",
        "format": "GeoTIFF",
        "max_items": 10,
    })
    vrt = block("gdal.build_vrt", inputs=[bare(read_coll)], args={"resolution": "highest"})
    paths.append(save(pipeline("gdal_build_vrt", "Build VRT from raster collection", [read_coll, vrt])))

    # 5. add_overviews
    read = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    ovr = block("gdal.add_overviews", inputs=[bare(read)], args={"levels": "2 4", "resampling": "average"})
    paths.append(save(pipeline("gdal_add_overviews", "Add pyramid overviews to raster", [read, ovr])))

    # 6. tile_index
    read_coll = block("data.read_collection", args={
        "uri": fp("") + "test_raster*.tif",
        "format": "GeoTIFF",
        "max_items": 10,
    })
    idx = block("gdal.tile_index", inputs=[bare(read_coll)])
    paths.append(save(pipeline("gdal_tile_index", "Build raster tile index", [read_coll, idx])))

    # 7. nearblack
    read = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    nb = block("gdal.nearblack", inputs=[bare(read)], args={"near": 15})
    paths.append(save(pipeline("gdal_nearblack", "Clean near-black edge pixels", [read, nb])))

    # 8. tile  (gdal2tiles requires 8-bit input, so translate to Byte first)
    read = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    byte = block("gdal.translate", inputs=[bare(read)], args={"output_type": "Byte"})
    tl = block("gdal.tile", inputs=[bare(byte)], args={"zoom": "0-2", "profile": "mercator"})
    paths.append(save(pipeline("gdal_tile", "Translate to Byte then generate XYZ map tiles", [read, byte, tl])))

    # 9. retile
    read = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    rt = block("gdal.retile", inputs=[bare(read)], args={"tile_width": 32, "tile_height": 32})
    paths.append(save(pipeline("gdal_retile", "Split raster into 32x32 tiles", [read, rt])))

    # --- Raster <-> Vector conversion ---

    # 10. rasterize (chain: read vector -> translate raster -> rasterize to avoid ambiguity)
    read_vec = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    read_ras = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    # The resolver will match two file outputs to two file inputs.
    # Order in inputs list may affect matching; we provide explicit refs.
    rast = block("gdal.rasterize", inputs=[
        explicit(read_vec, "file", as_input="vectors"),
        explicit(read_ras, "file", as_input="reference"),
    ], args={"burn_value": 1})
    paths.append(save(pipeline("gdal_rasterize", "Burn vectors into raster", [read_vec, read_ras, rast])))

    # 11. polygonize
    read = block("data.read", args={"uri": fp("test_categorical.tif"), "format": "GeoTIFF"})
    poly = block("gdal.polygonize", inputs=[bare(read)], args={"band": 1, "field_name": "DN"})
    paths.append(save(pipeline("gdal_polygonize", "Polygonize categorical raster", [read, poly])))

    # 12. contour
    read = block("data.read", args={"uri": fp("test_dem.tif"), "format": "GeoTIFF"})
    ctr = block("gdal.contour", inputs=[bare(read)], args={"interval": 50, "field_name": "elev"})
    paths.append(save(pipeline("gdal_contour", "Generate contour lines from DEM", [read, ctr])))

    # --- Raster analysis ---

    # 13. calc
    read = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    calc = block("gdal.calc", inputs=[bare(read)], args={"expression": "A * 2 + 1", "output_type": "Float32"})
    paths.append(save(pipeline("gdal_calc", "Evaluate raster algebra expression", [read, calc])))

    # 14. sieve
    read = block("data.read", args={"uri": fp("test_categorical.tif"), "format": "GeoTIFF"})
    sv = block("gdal.sieve", inputs=[bare(read)], args={"threshold": 10, "connectedness": 4})
    paths.append(save(pipeline("gdal_sieve", "Remove small connected regions", [read, sv])))

    # 15. fill_nodata
    read = block("data.read", args={"uri": fp("test_nodata_raster.tif"), "format": "GeoTIFF"})
    fill = block("gdal.fill_nodata", inputs=[bare(read)], args={"max_distance": 50})
    paths.append(save(pipeline("gdal_fill_nodata", "Interpolate nodata pixels", [read, fill])))

    # 16. proximity
    read = block("data.read", args={"uri": fp("test_binary.tif"), "format": "GeoTIFF"})
    prox = block("gdal.proximity", inputs=[bare(read)], args={"distance_units": "pixel"})
    paths.append(save(pipeline("gdal_proximity", "Compute distance-to-features raster", [read, prox])))

    # 17. grid
    read = block("data.read", args={"uri": fp("test_points.csv"), "format": "CSV"})
    grd = block("gdal.grid", inputs=[bare(read)], args={
        "algorithm": "invdist:power=2.0",
        "width": 32,
        "height": 32,
    })
    paths.append(save(pipeline("gdal_grid", "Interpolate scattered points onto grid", [read, grd])))

    # 18. viewshed
    read = block("data.read", args={"uri": fp("test_dem.tif"), "format": "GeoTIFF"})
    vs = block("gdal.viewshed", inputs=[bare(read)], args={
        "observer_x": -122.25,
        "observer_y": 37.75,
        "observer_height": 1.6,
    })
    paths.append(save(pipeline("gdal_viewshed", "Compute viewshed from observer point", [read, vs])))

    # --- Clip and mask ---

    # 19. clip_raster_by_vector
    read_ras = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    read_vec = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    clip = block("gdal.clip_raster_by_vector", inputs=[
        explicit(read_ras, "file", as_input="source"),
        explicit(read_vec, "file", as_input="boundary"),
    ], args={"crop_to_cutline": True})
    paths.append(save(pipeline("gdal_clip_raster_by_vector", "Clip raster to vector boundary", [read_ras, read_vec, clip])))

    # 20. clip_raster_by_extent
    read = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    clip = block("gdal.clip_raster_by_extent", inputs=[bare(read)], args={
        "xmin": -122.4, "ymin": 37.6, "xmax": -122.1, "ymax": 37.9,
    })
    paths.append(save(pipeline("gdal_clip_raster_by_extent", "Clip raster to bounding box", [read, clip])))

    # --- Terrain analysis ---

    # 21. hillshade
    read = block("data.read", args={"uri": fp("test_dem.tif"), "format": "GeoTIFF"})
    hs = block("gdal.hillshade", inputs=[bare(read)], args={"azimuth": 315, "altitude": 45, "z_factor": 1})
    paths.append(save(pipeline("gdal_hillshade", "Generate hillshade from DEM", [read, hs])))

    # 22. slope
    read = block("data.read", args={"uri": fp("test_dem.tif"), "format": "GeoTIFF"})
    sl = block("gdal.slope", inputs=[bare(read)], args={"slope_format": "degree"})
    paths.append(save(pipeline("gdal_slope", "Compute slope from DEM", [read, sl])))

    # 23. aspect
    read = block("data.read", args={"uri": fp("test_dem.tif"), "format": "GeoTIFF"})
    asp = block("gdal.aspect", inputs=[bare(read)])
    paths.append(save(pipeline("gdal_aspect", "Compute aspect from DEM", [read, asp])))

    # 24. color_relief
    read_dem = block("data.read", args={"uri": fp("test_dem.tif"), "format": "GeoTIFF"})
    read_ramp = block("data.read", args={"uri": fp("color_ramp.txt")})
    cr = block("gdal.color_relief", inputs=[
        explicit(read_dem, "file", as_input="source"),
        explicit(read_ramp, "file", as_input="color_ramp"),
    ])
    paths.append(save(pipeline("gdal_color_relief", "Apply color ramp to DEM", [read_dem, read_ramp, cr])))

    # 25. TRI
    read = block("data.read", args={"uri": fp("test_dem.tif"), "format": "GeoTIFF"})
    tri = block("gdal.tri", inputs=[bare(read)], args={"algorithm": "Wilson"})
    paths.append(save(pipeline("gdal_tri", "Compute Terrain Ruggedness Index", [read, tri])))

    # 26. TPI
    read = block("data.read", args={"uri": fp("test_dem.tif"), "format": "GeoTIFF"})
    tpi = block("gdal.tpi", inputs=[bare(read)])
    paths.append(save(pipeline("gdal_tpi", "Compute Topographic Position Index", [read, tpi])))

    # 27. roughness
    read = block("data.read", args={"uri": fp("test_dem.tif"), "format": "GeoTIFF"})
    rgh = block("gdal.roughness", inputs=[bare(read)])
    paths.append(save(pipeline("gdal_roughness", "Compute terrain roughness", [read, rgh])))

    # 28. terrain chain: DEM -> hillshade + slope + aspect (parallel from same source)
    read = block("data.read", args={"uri": fp("test_dem.tif"), "format": "GeoTIFF"})
    hs = block("gdal.hillshade", inputs=[bare(read)], args={"azimuth": 315, "altitude": 45})
    sl = block("gdal.slope", inputs=[bare(read)], args={"slope_format": "degree"})
    asp = block("gdal.aspect", inputs=[bare(read)])
    paths.append(save(pipeline(
        "gdal_terrain_parallel",
        "Parallel terrain analysis: hillshade + slope + aspect from single DEM",
        [read, hs, sl, asp],
    )))

    # --- Raster information ---

    # 29. info
    read = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    info = block("gdal.info", inputs=[bare(read)], args={"compute_stats": True})
    paths.append(save(pipeline("gdal_info", "Report raster metadata as JSON", [read, info])))

    # 30. location_info
    read = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    loc = block("gdal.location_info", inputs=[bare(read)], args={
        "x": -122.25, "y": 37.75, "coord_system": "georef",
    })
    paths.append(save(pipeline("gdal_location_info", "Read raster values at a point", [read, loc])))

    # 31. compare
    read1 = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    read2 = block("data.read", args={"uri": fp("test_raster2.tif"), "format": "GeoTIFF"})
    cmp = block("gdal.compare", inputs=[explicit(read1, "file", as_input="golden"), explicit(read2, "file", as_input="new")])
    paths.append(save(pipeline("gdal_compare", "Compare two rasters", [read1, read2, cmp])))

    # --- Vector I/O ---

    # 32. vector_translate
    read = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    vt = block("gdal.vector_translate", inputs=[bare(read)], args={
        "output_format": "GeoJSON",
        "target_crs": "EPSG:32610",
    })
    paths.append(save(pipeline("gdal_vector_translate", "Reproject vector to UTM", [read, vt])))

    # 33. vector_merge
    read_coll = block("data.read_collection", args={
        "uri": fp("") + "test_vectors*.geojson",
        "format": "GeoJSON",
        "max_items": 10,
    })
    vm = block("gdal.vector_merge", inputs=[bare(read_coll)])
    paths.append(save(pipeline("gdal_vector_merge", "Merge vector collection", [read_coll, vm])))

    # 34. vector_tile_index
    read_coll = block("data.read_collection", args={
        "uri": fp("") + "test_vectors*.geojson",
        "format": "GeoJSON",
        "max_items": 10,
    })
    vti = block("gdal.vector_tile_index", inputs=[bare(read_coll)])
    paths.append(save(pipeline("gdal_vector_tile_index", "Build vector footprint index", [read_coll, vti])))

    # --- Vector analysis (overlay operations) ---
    # For blocks with two file inputs, we use explicit references.
    # The resolver matches by type; since both inputs are type=file,
    # the match order depends on Go map iteration, but for testing
    # this is acceptable (overlay results are still valid).

    # 35. vector_union
    read_a = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    read_b = block("data.read", args={"uri": fp("test_vectors2.geojson"), "format": "GeoJSON"})
    vu = block("gdal.vector_union", inputs=[explicit(read_a, "file", as_input="a"), explicit(read_b, "file", as_input="b")])
    paths.append(save(pipeline("gdal_vector_union", "Union of two vector layers", [read_a, read_b, vu])))

    # 36. vector_intersection
    read_a = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    read_b = block("data.read", args={"uri": fp("test_vectors2.geojson"), "format": "GeoJSON"})
    vi = block("gdal.vector_intersection", inputs=[explicit(read_a, "file", as_input="a"), explicit(read_b, "file", as_input="b")])
    paths.append(save(pipeline("gdal_vector_intersection", "Intersection of two vector layers", [read_a, read_b, vi])))

    # 37. vector_difference
    read_a = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    read_b = block("data.read", args={"uri": fp("test_vectors2.geojson"), "format": "GeoJSON"})
    vd = block("gdal.vector_difference", inputs=[explicit(read_a, "file", as_input="a"), explicit(read_b, "file", as_input="b")])
    paths.append(save(pipeline("gdal_vector_difference", "Difference A \\ B", [read_a, read_b, vd])))

    # 38. vector_sym_difference
    read_a = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    read_b = block("data.read", args={"uri": fp("test_vectors2.geojson"), "format": "GeoJSON"})
    vsd = block("gdal.vector_sym_difference", inputs=[explicit(read_a, "file", as_input="a"), explicit(read_b, "file", as_input="b")])
    paths.append(save(pipeline("gdal_vector_sym_difference", "Symmetric difference of two layers", [read_a, read_b, vsd])))

    # 39. vector_identity
    read_a = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    read_b = block("data.read", args={"uri": fp("test_vectors2.geojson"), "format": "GeoJSON"})
    vid = block("gdal.vector_identity", inputs=[explicit(read_a, "file", as_input="a"), explicit(read_b, "file", as_input="b")])
    paths.append(save(pipeline("gdal_vector_identity", "Identity overlay A with B", [read_a, read_b, vid])))

    # 40. vector_clip
    read_a = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    read_b = block("data.read", args={"uri": fp("test_vectors2.geojson"), "format": "GeoJSON"})
    vc = block("gdal.vector_clip", inputs=[explicit(read_a, "file", as_input="a"), explicit(read_b, "file", as_input="b")])
    paths.append(save(pipeline("gdal_vector_clip", "Clip vector A by footprint of B", [read_a, read_b, vc])))

    # 41. vector_erase
    read_a = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    read_b = block("data.read", args={"uri": fp("test_vectors2.geojson"), "format": "GeoJSON"})
    ve = block("gdal.vector_erase", inputs=[explicit(read_a, "file", as_input="a"), explicit(read_b, "file", as_input="b")])
    paths.append(save(pipeline("gdal_vector_erase", "Erase features of A by B", [read_a, read_b, ve])))

    # 42. vector_update
    read_a = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    read_b = block("data.read", args={"uri": fp("test_vectors2.geojson"), "format": "GeoJSON"})
    vup = block("gdal.vector_update", inputs=[explicit(read_a, "file", as_input="a"), explicit(read_b, "file", as_input="b")])
    paths.append(save(pipeline("gdal_vector_update", "Update layer A with features from B", [read_a, read_b, vup])))

    # --- Vector info ---

    # 43. vector_info
    read = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    vi = block("gdal.vector_info", inputs=[bare(read)])
    paths.append(save(pipeline("gdal_vector_info", "Report vector metadata as JSON", [read, vi])))

    # --- CRS and coordinate transforms ---

    # 44. srs_info (source block - all scalar inputs)
    srs = block("gdal.srs_info", args={"crs": "EPSG:4326"})
    paths.append(save(pipeline("gdal_srs_info", "Report CRS metadata", [srs])))

    # 45. transform_points
    read = block("data.read", args={"uri": fp("test_points.csv"), "format": "CSV"})
    tp = block("gdal.transform_points", inputs=[bare(read)], args={
        "source_crs": "EPSG:4326",
        "target_crs": "EPSG:32610",
    })
    paths.append(save(pipeline("gdal_transform_points", "Transform point coordinates between CRSs", [read, tp])))

    # --- Map/Reduce ---

    # 46. map_raster_tiles -> warp -> reduce_mosaic
    read_coll = block("data.read_collection", args={
        "uri": fp("") + "test_raster*.tif",
        "format": "GeoTIFF",
        "max_items": 10,
    })
    mrt = block("gdal.map_raster_tiles", inputs=[bare(read_coll)])
    wrp = block("gdal.warp", inputs=[bare(mrt)], args={"target_crs": "EPSG:32610", "resampling": "bilinear"})
    red = block("gdal.reduce_mosaic", inputs=[bare(wrp)], args={"resampling": "nearest"})
    paths.append(save(pipeline(
        "gdal_map_warp_reduce_mosaic",
        "Map raster tiles, reproject each, mosaic back",
        [read_coll, mrt, wrp, red],
    )))

    # 47. map_raster_tiles -> translate -> reduce_vrt
    read_coll = block("data.read_collection", args={
        "uri": fp("") + "test_raster*.tif",
        "format": "GeoTIFF",
        "max_items": 10,
    })
    mrt = block("gdal.map_raster_tiles", inputs=[bare(read_coll)])
    xlat = block("gdal.translate", inputs=[bare(mrt)], args={"output_type": "Byte"})
    red = block("gdal.reduce_vrt", inputs=[bare(xlat)])
    paths.append(save(pipeline(
        "gdal_map_translate_reduce_vrt",
        "Map raster tiles, translate each to Byte, build VRT",
        [read_coll, mrt, xlat, red],
    )))

    # --- Complex chains ---

    # 48. raster processing chain
    read = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    wrp = block("gdal.warp", inputs=[bare(read)], args={"target_crs": "EPSG:32610", "resampling": "bilinear"})
    xlat = block("gdal.translate", inputs=[bare(wrp)], args={"output_type": "Byte"})
    info = block("gdal.info", inputs=[bare(xlat)], args={"compute_stats": True})
    paths.append(save(pipeline(
        "gdal_raster_chain",
        "Read -> warp -> translate -> info chain",
        [read, wrp, xlat, info],
    )))

    # 49. vector processing chain
    read = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    vt = block("gdal.vector_translate", inputs=[bare(read)], args={
        "output_format": "GeoJSON",
        "target_crs": "EPSG:32610",
    })
    vi = block("gdal.vector_info", inputs=[bare(vt)])
    paths.append(save(pipeline(
        "gdal_vector_chain",
        "Read -> vector_translate -> vector_info chain",
        [read, vt, vi],
    )))

    # 50. DEM -> contour -> vector_translate chain
    read = block("data.read", args={"uri": fp("test_dem.tif"), "format": "GeoTIFF"})
    ctr = block("gdal.contour", inputs=[bare(read)], args={"interval": 100})
    vt = block("gdal.vector_translate", inputs=[bare(ctr)], args={
        "output_format": "GeoJSON",
        "target_crs": "EPSG:32610",
    })
    paths.append(save(pipeline(
        "gdal_dem_contour_reproject",
        "DEM -> contour lines -> reproject to UTM",
        [read, ctr, vt],
    )))

    return paths


# ---- Data collection pipelines ----

def _data_pipelines() -> list[str]:
    paths = []
    fp = fixture_path
    output_dir = str(ROOT / "output")

    # --- Generic I/O (local file:// - no network required) ---

    # 1. data.read
    read = block("data.read", args={"uri": fp("test_data.csv"), "format": "CSV"})
    paths.append(save(pipeline("data_read", "Read a single local file", [read])))

    # 2. data.write
    read = block("data.read", args={"uri": fp("test_data.csv"), "format": "CSV"})
    wr = block("data.write", inputs=[bare(read)], args={
        "uri": output_dir + "/data_write_test.csv",
        "overwrite": True,
    })
    paths.append(save(pipeline("data_write", "Read a file and write it back to output", [read, wr])))

    # 3. data.read_collection
    read_coll = block("data.read_collection", args={
        "uri": fp("") + "test_data*.csv",
        "format": "CSV",
        "max_items": 10,
    })
    paths.append(save(pipeline("data_read_collection", "Read collection of local CSV files", [read_coll])))

    # 4. data.write_collection
    read_coll = block("data.read_collection", args={
        "uri": fp("") + "test_data*.csv",
        "format": "CSV",
        "max_items": 10,
    })
    wr_coll = block("data.write_collection", inputs=[bare(read_coll)], args={
        "uri": output_dir + "/data_write_collection_test/",
        "overwrite": True,
    })
    paths.append(save(pipeline("data_write_collection", "Read and write a file collection", [read_coll, wr_coll])))

    # 5. data.list
    ls = block("data.list", args={"uri": fp(""), "recursive": False})
    paths.append(save(pipeline("data_list", "List files in fixtures directory", [ls])))

    # 6. data.stat
    st = block("data.stat", args={"uri": fp("test_data.csv")})
    paths.append(save(pipeline("data_stat", "Stat a local file", [st])))

    # 7. data.read -> data.write -> data.stat chain
    read = block("data.read", args={"uri": fp("test_raster.tif"), "format": "GeoTIFF"})
    wr = block("data.write", inputs=[bare(read)], args={
        "uri": output_dir + "/data_chain_test.tif",
        "overwrite": True,
    })
    paths.append(save(pipeline("data_read_write_chain", "Read raster, write it to output", [read, wr])))

    # --- Network-dependent (public dataset) pipelines ---
    # These require internet access and potentially API keys.
    # Kept with small/fast parameters for testing.

    # 8. census_acs (needs CENSUS_API_KEY)
    ca = block("data.census_acs", args={
        "year": 2022,
        "dataset": "acs5",
        "table": "B01003",
        "geography": "state:*",
    })
    paths.append(save(pipeline("data_census_acs", "[NETWORK] Query Census ACS population data", [ca])))

    # 9. census_tiger
    ct = block("data.census_tiger", args={
        "year": 2022,
        "layer": "states",
    })
    paths.append(save(pipeline("data_census_tiger", "[NETWORK] Download TIGER/Line state boundaries", [ct])))

    # 10. fia (small: single state, one table)
    fia = block("data.fia", args={
        "state": "RI",
        "tables": "PLOT",
    })
    paths.append(save(pipeline("data_fia", "[NETWORK] Download FIA PLOT table for Rhode Island", [fia])))

    # 11. naturalearth_vector
    ne_v = block("data.naturalearth_vector", args={
        "scale": "110m",
        "category": "cultural",
        "theme": "admin_0_countries",
    })
    paths.append(save(pipeline("data_naturalearth_vector", "[NETWORK] Download Natural Earth country boundaries", [ne_v])))

    # 12. naturalearth_raster
    ne_r = block("data.naturalearth_raster", args={
        "scale": "50m",
        "theme": "HYP_HR_SR_OB_DR",
    })
    paths.append(save(pipeline("data_naturalearth_raster", "[NETWORK] Download Natural Earth hypsometric raster", [ne_r])))

    # 13. osm_extract_pbf (small region)
    osm_pbf = block("data.osm_extract_pbf", args={
        "region": "north-america/us/rhode-island",
    })
    paths.append(save(pipeline("data_osm_extract_pbf", "[NETWORK] Download OSM PBF for Rhode Island", [osm_pbf])))

    # 14. osm_extract_shp (small region)
    osm_shp = block("data.osm_extract_shp", args={
        "region": "north-america/us/rhode-island",
    })
    paths.append(save(pipeline("data_osm_extract_shp", "[NETWORK] Download OSM Shapefile for Rhode Island", [osm_shp])))

    # 15. nlcd (skip - very large download even for smallest product)
    nlcd = block("data.nlcd", args={
        "year": 2021,
        "product": "land_cover",
        "region": "HI",
    })
    paths.append(save(pipeline("data_nlcd", "[NETWORK] Download NLCD land cover for Hawaii", [nlcd])))

    # 16. prism (single month)
    prism = block("data.prism", args={
        "variable": "tmean",
        "start": "2023-01-01",
        "end": "2023-01-31",
        "resolution": "4km",
        "cadence": "monthly",
    })
    paths.append(save(pipeline("data_prism", "[NETWORK] Download PRISM monthly temperature", [prism])))

    # 17. ssurgo (small survey area)
    ssurgo = block("data.ssurgo", args={
        "area": "RI600",
    })
    paths.append(save(pipeline("data_ssurgo", "[NETWORK] Download SSURGO soils for RI survey area", [ssurgo])))

    # 18. usgs_3dep (needs an AOI file - chain with data.read)
    read_aoi = block("data.read", args={"uri": fp("test_vectors.geojson"), "format": "GeoJSON"})
    dep = block("data.usgs_3dep", inputs=[bare(read_aoi)], args={
        "resolution": "30m",
        "product": "DEM",
    })
    paths.append(save(pipeline("data_usgs_3dep", "[NETWORK] Fetch 3DEP DEM tiles for AOI", [read_aoi, dep])))

    # 19. nhd
    nhd = block("data.nhd", args={
        "huc": "0109",
        "resolution": "medium",
    })
    paths.append(save(pipeline("data_nhd", "[NETWORK] Download NHD for HUC-4 watershed", [nhd])))

    # --- Cross-collection integration pipelines ---
    # Note: data blocks that output directories (census_tiger,
    # naturalearth_vector, ssurgo, nhd, osm_extract_shp) cannot
    # chain directly into GDAL vector blocks that expect file-typed
    # inputs.  Cross-collection integration is already well covered
    # by the local pipelines that use data.read -> gdal.*.

    # 20. Census ACS -> base csv_to_parquet chain
    ca = block("data.census_acs", args={
        "year": 2022,
        "dataset": "acs5",
        "table": "B01003",
        "geography": "state:*",
    })
    to_pq = block("base.csv_to_parquet", inputs=[bare(ca)])
    paths.append(save(pipeline(
        "data_census_acs_to_parquet",
        "[NETWORK] Query Census ACS -> convert to Parquet",
        [ca, to_pq],
    )))

    # 21. Natural Earth raster -> GDAL info chain
    ne_r = block("data.naturalearth_raster", args={
        "scale": "50m",
        "theme": "HYP_HR_SR_OB_DR",
    })
    info = block("gdal.info", inputs=[bare(ne_r)], args={"compute_stats": True})
    paths.append(save(pipeline(
        "data_naturalearth_raster_info",
        "[NETWORK] Download Natural Earth raster -> GDAL info",
        [ne_r, info],
    )))

    return paths


# ---- Stats collection pipelines (R) ----

def _stats_pipelines() -> list[str]:
    paths = []
    fp = fixture_path
    csv = fp("stats_data.csv")

    def with_data(name: str, args: dict) -> dict:
        """Read the stats fixture CSV, then run a stats block over it."""
        read = block("data.read", args={"uri": csv, "format": "CSV"})
        blk = block(name, inputs=[bare(read)], args=args)
        return pipeline(
            name.replace(".", "_"),
            _read_desc(ROOT.parent / "blocks" / "stats" / "blocks"
                       / f"{name.split('.')[1]}.yaml"),
            [read, blk],
        )

    # 1. summary — descriptive statistics over the numeric columns
    paths.append(save(with_data("stats.summary", {"columns": "x,y"})))

    # 2. correlation — pearson correlation matrix of x and y
    paths.append(save(with_data(
        "stats.correlation", {"method": "pearson", "columns": "x,y"})))

    # 3. frequency — contingency table of group x region
    paths.append(save(with_data(
        "stats.frequency", {"column": "group", "by": "region"})))

    # 4. t_test — two-sample t-test of x across the two groups
    paths.append(save(with_data(
        "stats.t_test", {"value_column": "x", "group_column": "group"})))

    # 5. anova — one-way ANOVA of x across group
    paths.append(save(with_data(
        "stats.anova", {"value_column": "x", "group_column": "group"})))

    # 6. chisq_test — independence of group and region
    paths.append(save(with_data(
        "stats.chisq_test", {"column": "group", "by": "region"})))

    return paths


# ---- fiadb collection pipelines (TypeScript / Bun) ----
def _fiadb_pipelines() -> list[str]:
    """FIADB-API / EVALIDator estimate pipelines. All hit the live USFS service,
    so every pipeline is tagged [NETWORK]."""
    paths = []

    # 1. parameters — look up the snum attribute dictionary
    params = block("fiadb.parameters", args={"name": "snum"})
    paths.append(save(pipeline(
        "fiadb_parameters",
        "[NETWORK] Look up the FIADB-API snum attribute dictionary",
        [params],
    )))

    # 2. fullreport — area of forest land by county for Delaware 2020
    fr = block("fiadb.fullreport", args={
        "wc": "102020",
        "snum": "2",
        "rselected": "County code and name",
    })
    paths.append(save(pipeline(
        "fiadb_fullreport",
        "[NETWORK] FIA forest-land area by county for Delaware 2020",
        [fr],
    )))

    # 3. fullreport -> stats.summary chain: the estimate CSV feeds descriptive
    #    statistics, exercising cross-collection wiring (Bun -> R).
    fr2 = block("fiadb.fullreport", args={
        "wc": "102020",
        "snum": "2",
        "rselected": "County code and name",
    })
    summary = block("stats.summary", inputs=[bare(fr2)], args={
        "columns": "ESTIMATE,VARIANCE,SE,SE_PERCENT,PLOT_COUNT",
    })
    paths.append(save(pipeline(
        "fiadb_fullreport_summary",
        "[NETWORK] FIA county estimates -> stats.summary descriptive statistics",
        [fr2, summary],
    )))

    # ---- Friendly per-attribute blocks (no snum/wc/LABEL_VAR knowledge needed) ----

    # 4. biomass by county for Maine, converted to SI (auto-resolve latest eval)
    bio = block("fiadb.biomass", args={
        "state": "ME",
        "group_by": "county",
        "units": "si",
    })
    paths.append(save(pipeline(
        "fiadb_biomass_county",
        "[NETWORK] Aboveground biomass by county for Maine, SI units",
        [bio],
    )))

    # 5. forest-land area for Rhode Island (ungrouped, imperial)
    area = block("fiadb.area", args={"state": "RI"})
    paths.append(save(pipeline(
        "fiadb_area_state",
        "[NETWORK] Forest-land area for Rhode Island",
        [area],
    )))

    # 6. carbon by ownership for Delaware
    carbon = block("fiadb.carbon", args={
        "state": "DE",
        "group_by": "ownership",
    })
    paths.append(save(pipeline(
        "fiadb_carbon_ownership",
        "[NETWORK] Aboveground carbon by ownership group for Delaware",
        [carbon],
    )))

    # 7. friendly biomass -> stats.summary chain (auto-resolution + Bun->R)
    bio2 = block("fiadb.biomass", args={"state": "ME", "group_by": "county"})
    bio_summary = block("stats.summary", inputs=[bare(bio2)], args={
        "columns": "ESTIMATE,VARIANCE,SE,SE_PERCENT,PLOT_COUNT",
    })
    paths.append(save(pipeline(
        "fiadb_biomass_summary",
        "[NETWORK] Friendly biomass block -> stats.summary descriptive statistics",
        [bio2, bio_summary],
    )))

    return paths


def _read_desc(path: Path) -> str:
    with open(path) as f:
        data = yaml.safe_load(f)
    return data.get("description", "")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    print("=== Spade Integration Test Generator ===\n")
    create_fixtures()
    print()
    paths = generate_pipelines()
    print()

    # Print summary by category
    local = [p for p in PIPELINES.glob("*.yaml") if "[NETWORK]" not in _read_desc(p)]
    network = [p for p in PIPELINES.glob("*.yaml") if "[NETWORK]" in _read_desc(p)]
    print(f"  Local pipelines:   {len(local)}")
    print(f"  Network pipelines: {len(network)}")
    print(f"  Total:             {len(local) + len(network)}")
