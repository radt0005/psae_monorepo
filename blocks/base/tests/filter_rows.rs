mod common;

use base::common::table;
use base::filter_rows;

use crate::common::{put_input, work_dir, write_params};

fn csv_input(name: &str, dir: &std::path::Path) {
    put_input(
        dir,
        name,
        "in.csv",
        b"id,state,age\n1,NY,25\n2,NY,40\n3,MI,35\n4,CA,30\n5,NY,55\n",
    );
}

#[test]
fn filters_numeric_predicate() {
    let dir = work_dir();
    csv_input("table", dir.path());
    write_params(dir.path(), "expression: age > 30\n");
    filter_rows::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/result/result.parquet");
    let df = table::read_table(out.to_str().unwrap()).unwrap();
    assert_eq!(df.height(), 3);
}

#[test]
fn filters_string_equality() {
    let dir = work_dir();
    csv_input("table", dir.path());
    write_params(dir.path(), "expression: \"state = 'NY'\"\n");
    filter_rows::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/result/result.parquet");
    let df = table::read_table(out.to_str().unwrap()).unwrap();
    assert_eq!(df.height(), 3);
}

#[test]
fn filters_boolean_combination() {
    let dir = work_dir();
    csv_input("table", dir.path());
    write_params(dir.path(), "expression: \"age > 30 AND state = 'NY'\"\n");
    filter_rows::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/result/result.parquet");
    let df = table::read_table(out.to_str().unwrap()).unwrap();
    assert_eq!(df.height(), 2);
}

#[test]
fn empty_expression_rejected() {
    let dir = work_dir();
    csv_input("table", dir.path());
    write_params(dir.path(), "expression: \"\"\n");
    let err = filter_rows::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().to_lowercase().contains("empty"));
}

#[test]
fn invalid_sql_rejected() {
    let dir = work_dir();
    csv_input("table", dir.path());
    write_params(dir.path(), "expression: \"age >>> 30\"\n");
    let err = filter_rows::run_in(dir.path()).unwrap_err();
    let msg = err.to_string().to_lowercase();
    assert!(msg.contains("invalid") || msg.contains("filter"));
}
