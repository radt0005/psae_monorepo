"""Block: gdal.clip_raster_by_vector — convenience wrapper around gdalwarp -cutline."""
from __future__ import annotations

from osgeo import gdal
from spade import RasterFile, VectorFile, run

from gdal_blocks._common import output_path


def handler(
    source: RasterFile,
    boundary: VectorFile,
    crop_to_cutline: bool = True,
    all_touched: bool = False,
) -> RasterFile:
    out = output_path("clipped", "tif")
    ds = gdal.Warp(
        out,
        source.path,
        format="GTiff",
        cutlineDSName=boundary.path,
        cropToCutline=crop_to_cutline,
        warpOptions=[f"CUTLINE_ALL_TOUCHED={str(all_touched).upper()}"],
    )
    if ds is None:
        raise RuntimeError("gdal.Warp failed during clip_raster_by_vector")
    ds = None
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
