mod common;

use std::fs;
use std::path::Path;

use base::common::{column_names, table};
use base::{csv_to_parquet, reduce_stack};

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
fn stacks_identical_schemas() {
    let a = make_parquet_from_csv(b"id,val\n1,a\n2,b\n");
    let b = make_parquet_from_csv(b"id,val\n3,c\n4,d\n");

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
    write_params(dir.path(), "strict: true\n");
    reduce_stack::run_in(dir.path()).unwrap();

    let df = table::read_table(
        dir.path()
            .join("outputs/result/result.parquet")
            .to_str()
            .unwrap(),
    )
    .unwrap();
    assert_eq!(df.height(), 4);
    assert_eq!(column_names(&df), ["id", "val"]);
}

#[test]
fn strict_rejects_schema_mismatch() {
    let a = make_parquet_from_csv(b"id,val\n1,a\n");
    let b = make_parquet_from_csv(b"id,extra\n2,z\n");

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
    write_params(dir.path(), "strict: true\n");
    let err = reduce_stack::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("schema"));
}

#[test]
fn non_strict_fills_missing_columns() {
    let a = make_parquet_from_csv(b"id,val\n1,a\n");
    let b = make_parquet_from_csv(b"id,extra\n2,z\n");

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
    // Non-strict is the default.
    reduce_stack::run_in(dir.path()).unwrap();

    let df = table::read_table(
        dir.path()
            .join("outputs/result/result.parquet")
            .to_str()
            .unwrap(),
    )
    .unwrap();
    assert_eq!(df.height(), 2);
    // union of columns
    let mut cols = column_names(&df);
    cols.sort();
    assert_eq!(cols, vec!["extra".to_string(), "id".into(), "val".into()]);
}
