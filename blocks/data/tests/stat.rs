mod common;

use std::fs;

use data::stat;

use crate::common::{work_dir, write_params};

#[test]
fn stat_existing_file() {
    let dir = work_dir();
    let src = dir.path().join("x.bin");
    fs::write(&src, b"123456").unwrap();
    let uri = format!("file://{}", src.to_string_lossy());
    write_params(dir.path(), &format!("uri: {uri}\n"));
    stat::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/json/metadata.json");
    let body = fs::read_to_string(&out).unwrap();
    let v: serde_json::Value = serde_json::from_str(&body).unwrap();
    assert_eq!(v["size"].as_u64().unwrap(), 6);
}

#[test]
fn stat_missing_file_errors() {
    let dir = work_dir();
    let missing = dir.path().join("nope.bin");
    let uri = format!("file://{}", missing.to_string_lossy());
    write_params(dir.path(), &format!("uri: {uri}\n"));
    let err = stat::run_in(dir.path()).unwrap_err();
    let msg = err.to_string().to_lowercase();
    assert!(
        msg.contains("not found") || msg.contains("notfound"),
        "unexpected: {msg}"
    );
}

#[test]
fn empty_uri_errors() {
    let dir = work_dir();
    write_params(dir.path(), "uri: ''\n");
    let err = stat::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("uri"));
}
