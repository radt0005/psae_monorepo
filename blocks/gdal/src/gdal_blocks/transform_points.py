"""Block: gdal.transform_points — coordinate transform over a CSV of points."""
from __future__ import annotations

import csv

from osgeo import osr
from spade import TabularFile, run

from gdal_blocks._common import output_path, srs_from_user


def handler(
    points: TabularFile,
    source_crs: str,
    target_crs: str,
    x_field: str = "x",
    y_field: str = "y",
) -> TabularFile:
    src_srs = srs_from_user(source_crs)
    dst_srs = srs_from_user(target_crs)
    # Use traditional (x, y) = (lon, lat) axis order regardless of authority
    # axis definitions so input CSVs always behave as users expect.
    src_srs.SetAxisMappingStrategy(osr.OAMS_TRADITIONAL_GIS_ORDER)
    dst_srs.SetAxisMappingStrategy(osr.OAMS_TRADITIONAL_GIS_ORDER)
    transform = osr.CoordinateTransformation(src_srs, dst_srs)

    out = output_path("transformed", "csv")
    with open(points.path, "r", newline="") as f:
        reader = csv.DictReader(f)
        if reader.fieldnames is None:
            raise ValueError("input CSV has no header row")
        if x_field not in reader.fieldnames or y_field not in reader.fieldnames:
            raise ValueError(
                f"CSV is missing required columns {x_field!r} and/or {y_field!r}"
            )
        has_z = "z" in reader.fieldnames
        rows = list(reader)

    with open(out, "w", newline="") as f:
        writer = csv.writer(f)
        header = [x_field, y_field] + (["z"] if has_z else [])
        writer.writerow(header)
        for row in rows:
            x = float(row[x_field])
            y = float(row[y_field])
            if has_z:
                z = float(row["z"])
                nx, ny, nz = transform.TransformPoint(x, y, z)
                writer.writerow([nx, ny, nz])
            else:
                nx, ny, _ = transform.TransformPoint(x, y)
                writer.writerow([nx, ny])

    return TabularFile(path=out)


if __name__ == "__main__":
    run(handler)
