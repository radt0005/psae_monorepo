mod common;

use std::fs;

use data::write_collection;

use crate::common::{put_input, work_dir, write_params};

#[test]
fn uploads_all_files_and_writes_receipts() {
    let dir = work_dir();
    put_input(dir.path(), "files", "a.txt", b"A");
    put_input(dir.path(), "files", "b.txt", b"B");
    put_input(dir.path(), "files", "c.txt", b"C");

    let dest = dir.path().join("dest");
    fs::create_dir_all(&dest).unwrap();
    let uri = format!("file://{}/", dest.to_string_lossy());
    write_params(
        dir.path(),
        &format!("uri: {uri}\noverwrite: false\n"),
    );

    write_collection::run_in(dir.path()).unwrap();

    assert!(dest.join("a.txt").exists());
    assert!(dest.join("b.txt").exists());
    assert!(dest.join("c.txt").exists());
    assert_eq!(fs::read(dest.join("a.txt")).unwrap(), b"A");

    let receipts_path = dir.path().join("outputs/json/receipts.json");
    assert!(receipts_path.exists());
    let body = fs::read_to_string(&receipts_path).unwrap();
    let arr: serde_json::Value = serde_json::from_str(&body).unwrap();
    assert_eq!(arr.as_array().unwrap().len(), 3);
}

#[test]
fn existing_destination_without_overwrite_fails() {
    let dir = work_dir();
    put_input(dir.path(), "files", "a.txt", b"new");
    let dest = dir.path().join("dest");
    fs::create_dir_all(&dest).unwrap();
    fs::write(dest.join("a.txt"), b"existing").unwrap();
    let uri = format!("file://{}/", dest.to_string_lossy());
    write_params(
        dir.path(),
        &format!("uri: {uri}\noverwrite: false\n"),
    );
    let err = write_collection::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("exists"));
}
