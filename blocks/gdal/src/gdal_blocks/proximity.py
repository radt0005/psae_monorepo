"""Block: gdal.proximity — wraps gdal.ComputeProximity."""
from __future__ import annotations

from osgeo import gdal
from spade import RasterFile, run

from gdal_blocks._common import output_path


def handler(
    source: RasterFile,
    target_values: str = "",
    distance_units: str = "pixel",
    max_distance: float = 0.0,
) -> RasterFile:
    if distance_units not in ("pixel", "georef"):
        raise ValueError("distance_units must be 'pixel' or 'georef'")

    src = gdal.Open(source.path)
    if src is None:
        raise RuntimeError(f"could not open {source.path}")
    src_band = src.GetRasterBand(1)

    out = output_path("proximity", "tif")
    driver = gdal.GetDriverByName("GTiff")
    dst = driver.Create(out, src.RasterXSize, src.RasterYSize, 1, gdal.GDT_Float32)
    dst.SetGeoTransform(src.GetGeoTransform())
    dst.SetProjection(src.GetProjection())
    dst_band = dst.GetRasterBand(1)

    options: list[str] = [f"DISTUNITS={distance_units.upper()}"]
    if target_values:
        options.append(f"VALUES={target_values}")
    if max_distance > 0:
        options.append(f"MAXDIST={max_distance}")

    gdal.ComputeProximity(src_band, dst_band, options)
    dst.FlushCache()
    dst = None
    src = None
    return RasterFile(path=out)


if __name__ == "__main__":
    run(handler)
