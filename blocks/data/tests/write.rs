mod common;

use std::fs;

use data::write;

use crate::common::{put_input, work_dir, write_params};

#[test]
fn write_local_file_happy_path() {
    let dir = work_dir();
    put_input(dir.path(), "file", "src.bin", b"hello");
    let dest = dir.path().join("dest.bin");
    let uri = format!("file://{}", dest.to_string_lossy());
    write_params(
        dir.path(),
        &format!("uri: {uri}\noverwrite: false\n"),
    );

    write::run_in(dir.path()).unwrap();

    assert!(dest.exists());
    assert_eq!(fs::read(&dest).unwrap(), b"hello");

    let receipt_path = dir.path().join("outputs/json/receipt.json");
    assert!(receipt_path.exists());
    let body = fs::read_to_string(&receipt_path).unwrap();
    assert!(body.contains("\"bytes\""));
    assert!(body.contains("\"sha256\""));
}

#[test]
fn refuses_existing_destination_without_overwrite() {
    let dir = work_dir();
    put_input(dir.path(), "file", "src.bin", b"payload");
    let dest = dir.path().join("dest.bin");
    fs::write(&dest, b"existing").unwrap();
    let uri = format!("file://{}", dest.to_string_lossy());
    write_params(
        dir.path(),
        &format!("uri: {uri}\noverwrite: false\n"),
    );

    let err = write::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("exists"));
    // Existing file untouched
    assert_eq!(fs::read(&dest).unwrap(), b"existing");
}

#[test]
fn overwrite_true_replaces_destination() {
    let dir = work_dir();
    put_input(dir.path(), "file", "src.bin", b"new contents");
    let dest = dir.path().join("dest.bin");
    fs::write(&dest, b"old").unwrap();
    let uri = format!("file://{}", dest.to_string_lossy());
    write_params(
        dir.path(),
        &format!("uri: {uri}\noverwrite: true\n"),
    );

    write::run_in(dir.path()).unwrap();
    assert_eq!(fs::read(&dest).unwrap(), b"new contents");
}

#[test]
fn empty_uri_errors() {
    let dir = work_dir();
    put_input(dir.path(), "file", "src.bin", b"x");
    write_params(dir.path(), "uri: ''\noverwrite: false\n");
    let err = write::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("uri"));
}
