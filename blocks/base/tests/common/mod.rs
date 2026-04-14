//! Shared test helpers for integration tests.
#![allow(dead_code)]

use std::fs;
use std::path::{Path, PathBuf};

use tempfile::TempDir;

/// Build a fresh working directory with `inputs/`, `outputs/`, and `logs/`
/// subdirectories, ready to drive a block's `run_in(path)` entrypoint.
pub fn work_dir() -> TempDir {
    let dir = TempDir::new().expect("tempdir");
    for sub in ["inputs", "outputs", "logs"] {
        fs::create_dir_all(dir.path().join(sub)).unwrap();
    }
    dir
}

/// Write `params.yaml` in the working directory.
pub fn write_params(dir: &Path, content: &str) {
    fs::write(dir.join("params.yaml"), content).unwrap();
}

/// Create `inputs/<name>/<filename>` with the given contents.
pub fn put_input(dir: &Path, input_name: &str, filename: &str, content: &[u8]) -> PathBuf {
    let input_dir = dir.join("inputs").join(input_name);
    fs::create_dir_all(&input_dir).unwrap();
    let full = input_dir.join(filename);
    fs::write(&full, content).unwrap();
    full
}
