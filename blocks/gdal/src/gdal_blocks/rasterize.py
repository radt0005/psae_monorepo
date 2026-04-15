"""Block: gdal.rasterize — wraps gdal.Rasterize."""
from __future__ import annotations

from osgeo import gdal
from spade import RasterFile, VectorFile, run

from gdal_blocks._common import output_path


def handler(
    vectors: VectorFile,
    reference: RasterFile,
    burn_value: float = 1.0,
    attribute: str = "",
    all_touched: bool = False,
) -> RasterFile:
    ref = gdal.Open(reference.path)
    if ref is None:
        raise RuntimeError(f"could not open reference raster {reference.path}")

    gt = ref.GetGeoTransform()
    xres, yres = gt[1], -gt[5]
    xmin = gt[0]
    ymax = gt[3]
    xmax = xmin + gt[1] * ref.RasterXSize
    ymin = ymax + gt[5] * ref.RasterYSize

    out = output_path("rasterized", "tif")
    kwargs: dict = {
        "format": "GTiff",
        "outputType": gdal.GDT_Float32,
        "xRes": xres,
        "yRes": yres,
        "outputBounds": [xmin, ymin, xmax, ymax],
        "allTouched": all_touched,
        "initValues": 0,
    }
    if attribute:
        kwargs["attribute"] = attribute
    else:
        kwargs["burnValues"] = [burn_value]
    if ref.GetProjection():
        kwargs["outputSRS"] = ref.GetProjection()

    ds = gdal.Rasterize(out, vectors.path, **kwargs)
    if ds is None:
        raise RuntimeError("gdal.Rasterize failed")
    ds = None  # close
    ref = None
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
