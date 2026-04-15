"""Tests for vector_info (Phase 2.9)."""
from __future__ import annotations

import json
from pathlib import Path

from spade import VectorFile

from gdal_blocks import vector_info


def test_vector_info_reports_layer(vector_path: Path):
    result = vector_info.handler(VectorFile(path=str(vector_path)))
    data = json.loads(Path(result.path).read_text())
    # gdal.VectorInfo returns either a string report or dict; JSON mode produces dict.
    assert isinstance(data, dict)
    assert "layers" in data or "driverShortName" in data
