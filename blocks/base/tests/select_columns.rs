mod common;

use base::common::{column_names, table};
use base::select_columns;

use crate::common::{put_input, work_dir, write_params};

fn csv_input(dir: &std::path::Path) {
    put_input(
        dir,
        "table",
        "in.csv",
        b"id,name,score,rank\n1,alice,9.5,1\n2,bob,7.0,3\n",
    );
}

#[test]
fn keep_subset() {
    let dir = work_dir();
    csv_input(dir.path());
    write_params(dir.path(), "columns: \"id,score\"\n");
    select_columns::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/result/result.parquet");
    let df = table::read_table(out.to_str().unwrap()).unwrap();
    assert_eq!(column_names(&df), ["id", "score"]);
}

#[test]
fn drop_subset() {
    let dir = work_dir();
    csv_input(dir.path());
    write_params(dir.path(), "columns: \"score\"\nmode: drop\n");
    select_columns::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/result/result.parquet");
    let df = table::read_table(out.to_str().unwrap()).unwrap();
    assert_eq!(column_names(&df), ["id", "name", "rank"]);
}

#[test]
fn unknown_column_errors() {
    let dir = work_dir();
    csv_input(dir.path());
    write_params(dir.path(), "columns: \"id,nonexistent\"\n");
    let err = select_columns::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("unknown columns"));
}

#[test]
fn bad_mode_rejected() {
    let dir = work_dir();
    csv_input(dir.path());
    write_params(dir.path(), "columns: \"id\"\nmode: invert\n");
    let err = select_columns::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("mode"));
}
