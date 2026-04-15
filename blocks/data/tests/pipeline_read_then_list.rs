//! Cross-block pipeline test: `read_collection` → iterate output.
//!
//! This stitches two generic blocks together in memory, ensuring that
//! the output of one is in the shape the next expects.

mod common;

use std::fs;

use data::{read, read_collection};

use crate::common::{work_dir, write_params};

#[test]
fn read_collection_then_count_outputs_matches_source() {
    let src_dir = work_dir();
    let src = src_dir.path().join("src");
    fs::create_dir_all(&src).unwrap();
    for i in 0..4 {
        fs::write(src.join(format!("{i}.csv")), format!("row{i}\n")).unwrap();
    }

    let dir = work_dir();
    let uri = format!("file://{}/", src.to_string_lossy());
    write_params(
        dir.path(),
        &format!("uri: {uri}\nformat: CSV\nmax_items: 0\n"),
    );
    read_collection::run_in(dir.path()).unwrap();

    let files_dir = dir.path().join("outputs/files");
    let count = fs::read_dir(&files_dir).unwrap().count();
    assert_eq!(count, 4);
}

#[test]
fn read_single_then_sha256_matches_source() {
    // Use `read` to fetch a tempfile, then compute the sha256 of the
    // output and compare it against the source.
    let dir = work_dir();
    let src = dir.path().join("payload.bin");
    let payload = b"this is the payload";
    fs::write(&src, payload).unwrap();
    let uri = format!("file://{}", src.to_string_lossy());
    write_params(dir.path(), &format!("uri: {uri}\nformat: ''\n"));
    read::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/file/payload.bin");
    assert!(out.exists());
    let got = fs::read(&out).unwrap();
    assert_eq!(got, payload);
}
