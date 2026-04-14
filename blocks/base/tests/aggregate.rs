mod common;

use base::aggregate;
use base::common::{column_names, table};

use crate::common::{put_input, work_dir, write_params};

fn csv_input(dir: &std::path::Path) {
    put_input(
        dir,
        "table",
        "in.csv",
        b"state,score\nNY,10\nNY,20\nMI,5\nMI,15\nCA,30\nCA,40\nCA,50\n",
    );
}

#[test]
fn ungrouped_mean() {
    let dir = work_dir();
    csv_input(dir.path());
    write_params(
        dir.path(),
        "aggregations: '[{\"column\":\"score\",\"function\":\"mean\",\"as\":\"m\"}]'\n",
    );
    aggregate::run_in(dir.path()).unwrap();

    let df = table::read_table(
        dir.path()
            .join("outputs/result/result.parquet")
            .to_str()
            .unwrap(),
    )
    .unwrap();
    assert_eq!(df.height(), 1);
    assert_eq!(column_names(&df), ["m"]);
}

#[test]
fn grouped_mean_and_count() {
    let dir = work_dir();
    csv_input(dir.path());
    write_params(
        dir.path(),
        "aggregations: '[{\"column\":\"score\",\"function\":\"mean\",\"as\":\"avg\"},{\"column\":\"score\",\"function\":\"count\",\"as\":\"n\"}]'\ngroup_by: state\n",
    );
    aggregate::run_in(dir.path()).unwrap();

    let df = table::read_table(
        dir.path()
            .join("outputs/result/result.parquet")
            .to_str()
            .unwrap(),
    )
    .unwrap();
    assert_eq!(df.height(), 3);
    let cols = column_names(&df);
    assert!(cols.iter().any(|c| c == "state"));
    assert!(cols.iter().any(|c| c == "avg"));
    assert!(cols.iter().any(|c| c == "n"));
}

#[test]
fn percentile_requires_p() {
    let dir = work_dir();
    csv_input(dir.path());
    write_params(
        dir.path(),
        "aggregations: '[{\"column\":\"score\",\"function\":\"percentile\"}]'\n",
    );
    let err = aggregate::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("requires 'p'"));
}

#[test]
fn percentile_out_of_range() {
    let dir = work_dir();
    csv_input(dir.path());
    write_params(
        dir.path(),
        "aggregations: '[{\"column\":\"score\",\"function\":\"percentile\",\"p\":1.5}]'\n",
    );
    let err = aggregate::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("[0, 1]"));
}

#[test]
fn percentile_95() {
    let dir = work_dir();
    csv_input(dir.path());
    write_params(
        dir.path(),
        "aggregations: '[{\"column\":\"score\",\"function\":\"percentile\",\"p\":0.95,\"as\":\"p95\"}]'\n",
    );
    aggregate::run_in(dir.path()).unwrap();
    let df = table::read_table(
        dir.path()
            .join("outputs/result/result.parquet")
            .to_str()
            .unwrap(),
    )
    .unwrap();
    assert_eq!(column_names(&df), ["p95"]);
}

#[test]
fn unknown_function_rejected() {
    let dir = work_dir();
    csv_input(dir.path());
    write_params(
        dir.path(),
        "aggregations: '[{\"column\":\"score\",\"function\":\"bogus\"}]'\n",
    );
    let err = aggregate::run_in(dir.path()).unwrap_err();
    assert!(err
        .to_string()
        .to_lowercase()
        .contains("unknown aggregation"));
}

#[test]
fn malformed_json_rejected() {
    let dir = work_dir();
    csv_input(dir.path());
    write_params(dir.path(), "aggregations: \"not json\"\n");
    let err = aggregate::run_in(dir.path()).unwrap_err();
    assert!(
        err.to_string().to_lowercase().contains("json") || err.to_string().contains("expected")
    );
}

#[test]
fn count_and_count_distinct() {
    let dir = work_dir();
    csv_input(dir.path());
    write_params(
        dir.path(),
        "aggregations: '[{\"column\":\"state\",\"function\":\"count\",\"as\":\"total\"},{\"column\":\"state\",\"function\":\"count_distinct\",\"as\":\"distinct\"}]'\n",
    );
    aggregate::run_in(dir.path()).unwrap();
    let df = table::read_table(
        dir.path()
            .join("outputs/result/result.parquet")
            .to_str()
            .unwrap(),
    )
    .unwrap();
    assert_eq!(df.height(), 1);
}
