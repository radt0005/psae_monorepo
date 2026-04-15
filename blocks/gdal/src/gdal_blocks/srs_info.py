"""Block: gdal.srs_info — report CRS metadata in multiple forms."""
from __future__ import annotations

import json

from spade import JsonFile, run

from gdal_blocks._common import output_path, srs_from_user


def handler(crs: str) -> JsonFile:
    if not crs:
        raise ValueError("srs_info: 'crs' is required")
    srs = srs_from_user(crs)

    data = {
        "input": crs,
        "authority": srs.GetAuthorityCode(None),
        "authority_name": srs.GetAuthorityName(None),
        "name": srs.GetName(),
        "is_geographic": bool(srs.IsGeographic()),
        "is_projected": bool(srs.IsProjected()),
        "wkt": srs.ExportToWkt(),
        "proj": srs.ExportToProj4(),
    }
    try:
        data["wkt2"] = srs.ExportToWkt(["FORMAT=WKT2_2019"])
    except RuntimeError:
        data["wkt2"] = None

    out = output_path("srs_info", "json")
    with open(out, "w") as f:
        json.dump(data, f, indent=2)
    return JsonFile(path=out)


if __name__ == "__main__":
    run(handler)
