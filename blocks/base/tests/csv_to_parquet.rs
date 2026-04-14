mod common;

use std::fs;

use base::common::{column_names, table};
use base::csv_to_parquet;

use crate::common::{put_input, work_dir, write_params};

#[test]
fn converts_simple_csv_to_parquet() {
    let dir = work_dir();
    put_input(
        dir.path(),
        "table",
        "in.csv",
        b"id,name,score\n1,alice,9.5\n2,bob,7.0\n3,carol,8.25\n",
    );
    csv_to_parquet::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/result/result.parquet");
    assert!(out.exists());
    let df = table::read_table(out.to_str().unwrap()).unwrap();
    assert_eq!(df.height(), 3);
    assert_eq!(column_names(&df), ["id", "name", "score"]);
}

#[test]
fn respects_delimiter_param() {
    let dir = work_dir();
    put_input(dir.path(), "table", "in.csv", b"a;b\n1;x\n2;y\n");
    write_params(dir.path(), "delimiter: \";\"\n");
    csv_to_parquet::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/result/result.parquet");
    let df = table::read_table(out.to_str().unwrap()).unwrap();
    assert_eq!(df.height(), 2);
    assert_eq!(column_names(&df), ["a", "b"]);
}

#[test]
fn rejects_multi_character_delimiter() {
    let dir = work_dir();
    put_input(dir.path(), "table", "in.csv", b"a,b\n1,2\n");
    write_params(dir.path(), "delimiter: \"ab\"\n");
    let err = csv_to_parquet::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("single ASCII character"));
    // Sanity: ensure outputs dir was not written.
    let output_result = dir.path().join("outputs/result/result.parquet");
    assert!(!output_result.exists());
    // Clean up any stray files
    let _ = fs::remove_dir_all(dir.path().join("outputs/result"));
}
