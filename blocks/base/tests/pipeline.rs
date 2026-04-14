//! Cross-block integration: chain several block handlers to simulate real
//! pipeline usage without a scheduler.
mod common;

use std::fs;
use std::path::Path;

use base::common::{column_names, table};
use base::{aggregate, csv_to_parquet, filter_rows, reduce_stack, select_columns};

use crate::common::{put_input, work_dir, write_params};

fn copy_table_between(from: &Path, to_dir: &Path, as_input: &str, filename: &str) {
    let dest = to_dir.join("inputs").join(as_input);
    fs::create_dir_all(&dest).unwrap();
    fs::copy(from, dest.join(filename)).unwrap();
}

#[test]
fn csv_filter_select_aggregate_chain() {
    // Step 1: csv_to_parquet
    let s1 = work_dir();
    put_input(
        s1.path(),
        "table",
        "in.csv",
        b"state,age,score\nNY,25,10\nNY,40,20\nMI,35,5\nCA,30,15\nNY,55,25\n",
    );
    csv_to_parquet::run_in(s1.path()).unwrap();
    let s1_out = s1.path().join("outputs/result/result.parquet");

    // Step 2: filter_rows age > 30
    let s2 = work_dir();
    copy_table_between(&s1_out, s2.path(), "table", "in.parquet");
    write_params(s2.path(), "expression: age > 30\n");
    filter_rows::run_in(s2.path()).unwrap();
    let s2_out = s2.path().join("outputs/result/result.parquet");

    // Step 3: select_columns state,score
    let s3 = work_dir();
    copy_table_between(&s2_out, s3.path(), "table", "in.parquet");
    write_params(s3.path(), "columns: \"state,score\"\n");
    select_columns::run_in(s3.path()).unwrap();
    let s3_out = s3.path().join("outputs/result/result.parquet");

    // Step 4: aggregate mean(score) group by state
    let s4 = work_dir();
    copy_table_between(&s3_out, s4.path(), "table", "in.parquet");
    write_params(
        s4.path(),
        "aggregations: '[{\"column\":\"score\",\"function\":\"mean\",\"as\":\"avg\"}]'\ngroup_by: state\n",
    );
    aggregate::run_in(s4.path()).unwrap();
    let df = table::read_table(
        s4.path()
            .join("outputs/result/result.parquet")
            .to_str()
            .unwrap(),
    )
    .unwrap();
    // After filtering age>30, we have: NY(40,20), MI(35,5), NY(55,25) → groups NY and MI.
    assert_eq!(df.height(), 2);
    let cols = column_names(&df);
    assert!(cols.iter().any(|c| c == "state"));
    assert!(cols.iter().any(|c| c == "avg"));
}

#[test]
fn reduce_stack_after_selects() {
    // Simulate three mapped selects producing different subsets, then stacked.
    let parts = [
        b"id,kind\n1,alpha\n" as &[u8],
        b"id,kind\n2,beta\n",
        b"id,kind\n3,gamma\n",
    ];
    let stacks = work_dir();
    for (i, csv) in parts.iter().enumerate() {
        let s = work_dir();
        put_input(s.path(), "table", "in.csv", csv);
        csv_to_parquet::run_in(s.path()).unwrap();
        let src = s.path().join("outputs/result/result.parquet");
        let dest_dir = stacks.path().join("inputs/tables");
        fs::create_dir_all(&dest_dir).unwrap();
        fs::copy(&src, dest_dir.join(format!("{:02}.parquet", i))).unwrap();
    }
    reduce_stack::run_in(stacks.path()).unwrap();
    let df = table::read_table(
        stacks
            .path()
            .join("outputs/result/result.parquet")
            .to_str()
            .unwrap(),
    )
    .unwrap();
    assert_eq!(df.height(), 3);
    assert_eq!(column_names(&df), ["id", "kind"]);
}
