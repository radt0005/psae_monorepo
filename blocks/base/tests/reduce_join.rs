mod common;

use std::fs;
use std::path::Path;

use base::common::{column_names, table};
use base::{csv_to_parquet, reduce_join};

use crate::common::{put_input, work_dir, write_params};

fn make_parquet_from_csv(csv: &[u8]) -> tempfile::TempDir {
    let d = work_dir();
    put_input(d.path(), "table", "in.csv", csv);
    csv_to_parquet::run_in(d.path()).unwrap();
    d
}

fn copy_into_tables(dir: &Path, src: &Path, as_name: &str) {
    let dest_dir = dir.join("inputs/tables");
    fs::create_dir_all(&dest_dir).unwrap();
    fs::copy(src, dest_dir.join(as_name)).unwrap();
}

#[test]
fn inner_join_two_tables() {
    let a = make_parquet_from_csv(b"id,val_a\n1,x\n2,y\n3,z\n");
    let b = make_parquet_from_csv(b"id,val_b\n2,alpha\n3,beta\n4,gamma\n");

    let dir = work_dir();
    copy_into_tables(
        dir.path(),
        &a.path().join("outputs/result/result.parquet"),
        "01.parquet",
    );
    copy_into_tables(
        dir.path(),
        &b.path().join("outputs/result/result.parquet"),
        "02.parquet",
    );
    write_params(dir.path(), "on: id\nhow: inner\n");
    reduce_join::run_in(dir.path()).unwrap();

    let df = table::read_table(
        dir.path()
            .join("outputs/result/result.parquet")
            .to_str()
            .unwrap(),
    )
    .unwrap();
    assert_eq!(df.height(), 2);
}

#[test]
fn left_join_keeps_nulls() {
    let a = make_parquet_from_csv(b"id,val_a\n1,x\n2,y\n3,z\n");
    let b = make_parquet_from_csv(b"id,val_b\n2,alpha\n");

    let dir = work_dir();
    copy_into_tables(
        dir.path(),
        &a.path().join("outputs/result/result.parquet"),
        "01.parquet",
    );
    copy_into_tables(
        dir.path(),
        &b.path().join("outputs/result/result.parquet"),
        "02.parquet",
    );
    write_params(dir.path(), "on: id\nhow: left\n");
    reduce_join::run_in(dir.path()).unwrap();
    let df = table::read_table(
        dir.path()
            .join("outputs/result/result.parquet")
            .to_str()
            .unwrap(),
    )
    .unwrap();
    assert_eq!(df.height(), 3);
}

#[test]
fn chain_three_tables() {
    let a = make_parquet_from_csv(b"id,a\n1,X\n2,Y\n");
    let b = make_parquet_from_csv(b"id,b\n1,P\n2,Q\n");
    let c = make_parquet_from_csv(b"id,c\n1,alpha\n2,beta\n");

    let dir = work_dir();
    copy_into_tables(
        dir.path(),
        &a.path().join("outputs/result/result.parquet"),
        "01.parquet",
    );
    copy_into_tables(
        dir.path(),
        &b.path().join("outputs/result/result.parquet"),
        "02.parquet",
    );
    copy_into_tables(
        dir.path(),
        &c.path().join("outputs/result/result.parquet"),
        "03.parquet",
    );
    write_params(dir.path(), "on: id\n");
    reduce_join::run_in(dir.path()).unwrap();
    let df = table::read_table(
        dir.path()
            .join("outputs/result/result.parquet")
            .to_str()
            .unwrap(),
    )
    .unwrap();
    assert_eq!(df.height(), 2);
    let cols = column_names(&df);
    assert!(cols.iter().any(|c| c == "a"));
    assert!(cols.iter().any(|c| c == "b"));
    assert!(cols.iter().any(|c| c == "c"));
}

#[test]
fn missing_key_reports_clearly() {
    let a = make_parquet_from_csv(b"id,val_a\n1,x\n2,y\n");
    let b = make_parquet_from_csv(b"not_id,val_b\n1,alpha\n");

    let dir = work_dir();
    copy_into_tables(
        dir.path(),
        &a.path().join("outputs/result/result.parquet"),
        "01.parquet",
    );
    copy_into_tables(
        dir.path(),
        &b.path().join("outputs/result/result.parquet"),
        "02.parquet",
    );
    write_params(dir.path(), "on: id\n");
    let err = reduce_join::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("missing key"));
}

#[test]
fn unknown_how_rejected() {
    let a = make_parquet_from_csv(b"id,val_a\n1,x\n");
    let b = make_parquet_from_csv(b"id,val_b\n1,y\n");

    let dir = work_dir();
    copy_into_tables(
        dir.path(),
        &a.path().join("outputs/result/result.parquet"),
        "01.parquet",
    );
    copy_into_tables(
        dir.path(),
        &b.path().join("outputs/result/result.parquet"),
        "02.parquet",
    );
    write_params(dir.path(), "on: id\nhow: sideways\n");
    let err = reduce_join::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("unknown join"));
}
