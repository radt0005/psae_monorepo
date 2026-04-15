"""Block: gdal.tile — wraps osgeo_utils.gdal2tiles."""
from __future__ import annotations

from pathlib import Path

from osgeo_utils import gdal2tiles
from spade import Directory, RasterFile, run


def handler(
    source: RasterFile,
    zoom: str = "0-5",
    profile: str = "mercator",
    resampling: str = "average",
) -> Directory:
    out_dir = Path("tiles")
    out_dir.mkdir(parents=True, exist_ok=True)

    argv = [
        "gdal2tiles",
        "--profile", profile,
        "--resampling", resampling,
        "--zoom", zoom,
        "--webviewer=none",
        "--quiet",
        source.path,
        str(out_dir),
    ]
    gdal2tiles.main(argv)
    return Directory(path=str(out_dir))


if __name__ == "__main__":
    run(handler)
