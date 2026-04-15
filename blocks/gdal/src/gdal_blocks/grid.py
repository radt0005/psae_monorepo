"""Block: gdal.grid — wraps gdal.Grid."""
from __future__ import annotations

from pathlib import Path

from osgeo import gdal
from spade import TabularFile, run
from spade.types import RasterFile

from gdal_blocks._common import output_path


def _write_vrt(csv_path: str, x: str, y: str, z: str) -> str:
    """Create a sibling VRT descriptor for gdal.Grid to ingest the CSV."""
    vrt_path = str(Path(csv_path).with_suffix(".vrt"))
    layer_name = Path(csv_path).stem
    vrt = (
        f'<OGRVRTDataSource>\n'
        f'  <OGRVRTLayer name="{layer_name}">\n'
        f'    <SrcDataSource relativeToVRT="0">{csv_path}</SrcDataSource>\n'
        f'    <GeometryType>wkbPoint</GeometryType>\n'
        f'    <GeometryField encoding="PointFromColumns" x="{x}" y="{y}" z="{z}"/>\n'
        f'  </OGRVRTLayer>\n'
        f'</OGRVRTDataSource>\n'
    )
    Path(vrt_path).write_text(vrt)
    return vrt_path


def handler(
    points: TabularFile,
    algorithm: str = "invdist:power=2.0",
    width: int = 64,
    height: int = 64,
    z_field: str = "z",
    x_field: str = "x",
    y_field: str = "y",
) -> RasterFile:
    vrt_path = _write_vrt(points.path, x_field, y_field, z_field)
    out = output_path("gridded", "tif")

    ds = gdal.Grid(
        out,
        vrt_path,
        format="GTiff",
        width=width,
        height=height,
        zfield=z_field,
        algorithm=algorithm,
    )
    if ds is None:
        raise RuntimeError("gdal.Grid failed")
    ds = None
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
