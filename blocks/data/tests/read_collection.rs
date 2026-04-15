mod common;

use std::fs;

use data::read_collection;

use crate::common::{work_dir, write_params};

fn seed_dir(dir: &std::path::Path, files: &[(&str, &[u8])]) {
    fs::create_dir_all(dir).unwrap();
    for (name, bytes) in files {
        fs::write(dir.join(name), bytes).unwrap();
    }
}

#[test]
fn lists_five_files_in_prefix() {
    let dir = work_dir();
    let src = dir.path().join("src");
    seed_dir(
        &src,
        &[
            ("a.csv", b"1"),
            ("b.csv", b"2"),
            ("c.csv", b"3"),
            ("d.csv", b"4"),
            ("e.csv", b"5"),
        ],
    );
    let uri = format!("file://{}/", src.to_string_lossy());
    write_params(
        dir.path(),
        &format!("uri: {uri}\nformat: ''\nmax_items: 0\n"),
    );
    read_collection::run_in(dir.path()).unwrap();

    let out_dir = dir.path().join("outputs/files");
    let count = fs::read_dir(&out_dir).unwrap().count();
    assert_eq!(count, 5);
}

#[test]
fn glob_filters_csv_only() {
    let dir = work_dir();
    let src = dir.path().join("src");
    seed_dir(
        &src,
        &[
            ("a.csv", b"1"),
            ("b.csv", b"2"),
            ("c.txt", b"nope"),
        ],
    );
    let uri = format!("file://{}/*.csv", src.to_string_lossy());
    write_params(
        dir.path(),
        &format!("uri: {uri}\nformat: ''\nmax_items: 0\n"),
    );
    read_collection::run_in(dir.path()).unwrap();

    let out_dir = dir.path().join("outputs/files");
    let mut names: Vec<String> = fs::read_dir(&out_dir)
        .unwrap()
        .map(|e| e.unwrap().file_name().to_string_lossy().to_string())
        .collect();
    names.sort();
    assert_eq!(names.len(), 2);
    for n in names {
        assert!(n.ends_with(".csv"), "unexpected: {n}");
    }
}

#[test]
fn max_items_cap_errors() {
    let dir = work_dir();
    let src = dir.path().join("src");
    seed_dir(
        &src,
        &[
            ("a.csv", b"1"),
            ("b.csv", b"2"),
            ("c.csv", b"3"),
        ],
    );
    let uri = format!("file://{}/", src.to_string_lossy());
    write_params(
        dir.path(),
        &format!("uri: {uri}\nformat: ''\nmax_items: 2\n"),
    );
    let err = read_collection::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("max_items"));
}

#[test]
fn empty_prefix_empty_collection() {
    let dir = work_dir();
    let src = dir.path().join("empty");
    fs::create_dir_all(&src).unwrap();
    let uri = format!("file://{}/", src.to_string_lossy());
    write_params(
        dir.path(),
        &format!("uri: {uri}\nformat: ''\nmax_items: 0\n"),
    );
    read_collection::run_in(dir.path()).unwrap();

    let out_dir = dir.path().join("outputs/files");
    // Directory may exist (even empty) if the collection type created it.
    if out_dir.exists() {
        let count = fs::read_dir(&out_dir).unwrap().count();
        assert_eq!(count, 0);
    }
}

#[test]
fn rejects_double_star() {
    let dir = work_dir();
    let src = dir.path().join("src");
    fs::create_dir_all(&src).unwrap();
    let uri = format!("file://{}/**/*.csv", src.to_string_lossy());
    write_params(
        dir.path(),
        &format!("uri: {uri}\nformat: ''\nmax_items: 0\n"),
    );
    let err = read_collection::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("**"));
}
