"""Block: gdal.warp — wraps gdal.Warp."""
from __future__ import annotations

from osgeo import gdal
from spade import RasterFile, run

from gdal_blocks._common import output_path
from gdal_blocks._resampling import warp_method


def handler(
    source: RasterFile,
    target_crs: str = "",
    resolution: float = 0.0,
    resampling: str = "nearest",
    output_format: str = "GTiff",
) -> RasterFile:
    out = output_path("warped", "tif")

    kwargs: dict = {
        "format": output_format,
        "resampleAlg": warp_method(resampling),
    }
    if target_crs:
        kwargs["dstSRS"] = target_crs
    if resolution:
        kwargs["xRes"] = resolution
        kwargs["yRes"] = resolution

    ds = gdal.Warp(out, source.path, **kwargs)
    if ds is None:
        raise RuntimeError(f"gdal.Warp failed for {source.path}")
    ds = None  # close
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
