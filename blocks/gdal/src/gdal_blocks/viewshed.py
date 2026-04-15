"""Block: gdal.viewshed — wraps gdal.ViewshedGenerate."""
from __future__ import annotations

from osgeo import gdal
from spade import RasterFile, run

from gdal_blocks._common import output_path


def handler(
    source: RasterFile,
    observer_x: float,
    observer_y: float,
    observer_height: float = 1.6,
    target_height: float = 0.0,
    max_distance: float = 0.0,
) -> RasterFile:
    src = gdal.Open(source.path)
    if src is None:
        raise RuntimeError(f"could not open {source.path}")
    src_band = src.GetRasterBand(1)

    out = output_path("viewshed", "tif")
    ds = gdal.ViewshedGenerate(
        srcBand=src_band,
        driverName="GTiff",
        targetRasterName=out,
        creationOptions=[],
        observerX=observer_x,
        observerY=observer_y,
        observerHeight=observer_height,
        targetHeight=target_height,
        visibleVal=255.0,
        invisibleVal=0.0,
        outOfRangeVal=0.0,
        noDataVal=-1.0,
        dfCurvCoeff=0.85714,
        mode=1,  # GVM_Diagonal
        maxDistance=max_distance,
    )
    if ds is None:
        raise RuntimeError("gdal.ViewshedGenerate failed")
    ds = None
    src = None
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
