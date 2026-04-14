mod common;

use base::common::{column_names, table};
use base::group_by;

use crate::common::{put_input, work_dir, write_params};

#[test]
fn group_by_state_with_mean() {
    let dir = work_dir();
    put_input(
        dir.path(),
        "table",
        "in.csv",
        b"state,score\nNY,10\nNY,20\nMI,5\nMI,15\n",
    );
    write_params(
        dir.path(),
        "group_columns: state\naggregations: '[{\"column\":\"score\",\"function\":\"mean\",\"as\":\"m\"}]'\n",
    );
    group_by::run_in(dir.path()).unwrap();

    let df = table::read_table(
        dir.path()
            .join("outputs/result/result.parquet")
            .to_str()
            .unwrap(),
    )
    .unwrap();
    assert_eq!(df.height(), 2);
    let cols = column_names(&df);
    assert!(cols.iter().any(|c| c == "state"));
    assert!(cols.iter().any(|c| c == "m"));
}

#[test]
fn empty_group_columns_rejected() {
    let dir = work_dir();
    put_input(dir.path(), "table", "in.csv", b"a,b\n1,2\n");
    write_params(
        dir.path(),
        "group_columns: \"\"\naggregations: '[{\"column\":\"b\",\"function\":\"mean\"}]'\n",
    );
    let err = group_by::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("group_columns"));
}
