mod common;

use std::fs;

use base::reduce_collection;

use crate::common::{put_input, work_dir};

#[test]
fn passes_collection_through() {
    let dir = work_dir();
    put_input(dir.path(), "items", "a.txt", b"alpha");
    put_input(dir.path(), "items", "b.txt", b"beta");
    put_input(dir.path(), "items", "c.txt", b"gamma");

    reduce_collection::run_in(dir.path()).unwrap();

    let out_dir = dir.path().join("outputs/result");
    assert!(out_dir.exists());
    let mut files: Vec<_> = fs::read_dir(&out_dir)
        .unwrap()
        .filter_map(|e| e.ok())
        .map(|e| e.file_name().to_string_lossy().to_string())
        .collect();
    files.sort();
    assert_eq!(files, vec!["a.txt", "b.txt", "c.txt"]);

    assert_eq!(fs::read_to_string(out_dir.join("a.txt")).unwrap(), "alpha");
}
