"""Block: gdal.translate — wraps gdal.Translate."""
from __future__ import annotations

from osgeo import gdal
from spade import RasterFile, run

from gdal_blocks._common import output_path


_EXTENSIONS = {
    "GTiff": "tif",
    "COG": "tif",
    "HFA": "img",
    "VRT": "vrt",
    "NetCDF": "nc",
    "PNG": "png",
    "JPEG": "jpg",
}


def handler(
    source: RasterFile,
    output_format: str = "GTiff",
    output_type: str = "",
    width: int = 0,
    height: int = 0,
    scale_min: float | None = None,
    scale_max: float | None = None,
) -> RasterFile:
    ext = _EXTENSIONS.get(output_format, "tif")
    out = output_path("translated", ext)

    kwargs: dict = {"format": output_format}
    if output_type:
        kwargs["outputType"] = gdal.GetDataTypeByName(output_type)
    if width or height:
        kwargs["width"] = width
        kwargs["height"] = height
    if scale_min is not None and scale_max is not None:
        kwargs["scaleParams"] = [[scale_min, scale_max]]

    ds = gdal.Translate(out, source.path, **kwargs)
    if ds is None:
        raise RuntimeError(f"gdal.Translate failed for {source.path}")
    ds = None  # close
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
