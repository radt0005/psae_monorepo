//! Block: `base.group_by`
//!
//! Convenience wrapper around `base.aggregate` that takes the group columns
//! as a dedicated parameter. Delegates to the same core aggregation routine.

use std::path::{Path, PathBuf};

use spade::{Args, Outputs, TabularFile};

use crate::aggregate::{run_aggregations, AggregationSpec};
use crate::common::{params, table, BaseError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<Outputs> {
    let input: TabularFile = args.input("table")?;
    let group_columns_param: String = args.param("group_columns")?;
    let aggregations: String = args.param("aggregations")?;

    let group_cols = params::parse_csv_list(&group_columns_param);
    if group_cols.is_empty() {
        return Err(BaseError::BadExpression(
            "group_columns must not be empty".into(),
        ));
    }
    let specs: Vec<AggregationSpec> = params::parse_json_list(&aggregations)?;

    let lf = table::read_table_lazy(&input.path)?;
    let mut df = run_aggregations(lf, &group_cols, &specs)?;

    let out_path: PathBuf = base.join("result.parquet");
    table::write_parquet(&mut df, out_path.to_str().expect("utf-8 path"))?;

    let mut out = Outputs::new();
    out.add("result", TabularFile::new(out_path.to_string_lossy()));
    Ok(out)
}

pub fn entry() {
    let base = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
    spade::run(move |args| handler(args, &base).map_err(Into::into));
}

pub fn run_in(base: &Path) -> Result<()> {
    let args = spade::scanning::build_args_from(base)?;
    let out = handler(args, base)?;
    spade::output::write_outputs_to(out, base, None)?;
    Ok(())
}
