import os

import pytest
import yaml


@pytest.fixture
def work_dir(tmp_path):
    """Create a mock Spade working directory with inputs/, outputs/, logs/."""
    (tmp_path / "inputs").mkdir()
    (tmp_path / "outputs").mkdir()
    (tmp_path / "logs").mkdir()
    original = os.getcwd()
    os.chdir(tmp_path)
    yield tmp_path
    os.chdir(original)


@pytest.fixture
def write_params(work_dir):
    """Helper to write params.yaml content."""
    def _write(params: dict):
        with open(work_dir / "params.yaml", "w") as f:
            yaml.dump(params, f)
    return _write


@pytest.fixture
def create_input_file(work_dir):
    """Helper to create a single file in an input subdirectory."""
    def _create(name: str, filename: str = "data.tif", content: bytes = b"test data"):
        input_dir = work_dir / "inputs" / name
        input_dir.mkdir(exist_ok=True)
        file_path = input_dir / filename
        file_path.write_bytes(content)
        return file_path
    return _create


@pytest.fixture
def create_input_collection(work_dir):
    """Helper to create multiple files in an input subdirectory."""
    def _create(name: str, filenames: list[str], content: bytes = b"test data"):
        input_dir = work_dir / "inputs" / name
        input_dir.mkdir(exist_ok=True)
        paths = []
        for filename in filenames:
            file_path = input_dir / filename
            file_path.write_bytes(content)
            paths.append(file_path)
        return paths
    return _create
