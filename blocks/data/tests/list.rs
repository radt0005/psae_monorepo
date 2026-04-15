mod common;

use std::fs;

use data::list;

use crate::common::{work_dir, write_params};

#[test]
fn flat_listing_sorted() {
    let dir = work_dir();
    let src = dir.path().join("src");
    fs::create_dir_all(&src).unwrap();
    for name in ["c.txt", "a.txt", "b.txt"] {
        fs::write(src.join(name), b"x").unwrap();
    }

    let uri = format!("file://{}/", src.to_string_lossy());
    write_params(
        dir.path(),
        &format!("uri: {uri}\nrecursive: false\n"),
    );
    list::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/json/listing.json");
    let body = fs::read_to_string(&out).unwrap();
    let arr: Vec<serde_json::Value> = serde_json::from_str(&body).unwrap();
    assert_eq!(arr.len(), 3);
    let keys: Vec<String> = arr
        .iter()
        .map(|v| v["key"].as_str().unwrap().to_string())
        .collect();
    // Sorted
    let mut sorted = keys.clone();
    sorted.sort();
    assert_eq!(keys, sorted);
}

#[test]
fn empty_prefix_empty_listing() {
    let dir = work_dir();
    let src = dir.path().join("empty");
    fs::create_dir_all(&src).unwrap();
    let uri = format!("file://{}/", src.to_string_lossy());
    write_params(
        dir.path(),
        &format!("uri: {uri}\nrecursive: false\n"),
    );
    list::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/json/listing.json");
    let body = fs::read_to_string(&out).unwrap();
    let arr: Vec<serde_json::Value> = serde_json::from_str(&body).unwrap();
    assert_eq!(arr.len(), 0);
}

#[test]
fn empty_uri_errors() {
    let dir = work_dir();
    write_params(dir.path(), "uri: ''\nrecursive: false\n");
    let err = list::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("uri"));
}
