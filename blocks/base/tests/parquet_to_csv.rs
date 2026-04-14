mod common;

use std::fs;

use base::common::{column_names, table};
use base::{csv_to_parquet, parquet_to_csv};

use crate::common::{put_input, work_dir, write_params};

fn make_parquet(dir: &std::path::Path) -> std::path::PathBuf {
    // Build a parquet file via csv_to_parquet so we don't depend on polars
    // write APIs elsewhere.
    let w = work_dir();
    put_input(
        w.path(),
        "table",
        "in.csv",
        b"id,name,score\n1,alice,9.5\n2,bob,7.0\n",
    );
    csv_to_parquet::run_in(w.path()).unwrap();
    let src = w.path().join("outputs/result/result.parquet");
    let dest = dir.join("inputs/table/data.parquet");
    fs::create_dir_all(dest.parent().unwrap()).unwrap();
    fs::copy(&src, &dest).unwrap();
    dest
}

#[test]
fn converts_parquet_to_csv_round_trip() {
    let dir = work_dir();
    make_parquet(dir.path());
    parquet_to_csv::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/result/result.csv");
    assert!(out.exists());

    // Re-read the CSV and check schema + height.
    let df = table::read_table(out.to_str().unwrap()).unwrap();
    assert_eq!(df.height(), 2);
    assert_eq!(column_names(&df), ["id", "name", "score"]);
}

#[test]
fn respects_delimiter_param() {
    let dir = work_dir();
    make_parquet(dir.path());
    write_params(dir.path(), "delimiter: \";\"\n");
    parquet_to_csv::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/result/result.csv");
    let body = fs::read_to_string(&out).unwrap();
    assert!(body.contains("id;name;score"));
}
