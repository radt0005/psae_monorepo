//! Block: `base.select_columns`
//!
//! Project a subset of columns from a table, keeping or dropping them.

use std::path::{Path, PathBuf};

use polars::prelude::*;
use spade::{Args, Outputs, TabularFile};

use crate::common::{params, table, BaseError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<Outputs> {
    let input: TabularFile = args.input("table")?;
    let columns_param: String = args.param("columns")?;
    let mode: String = args
        .param::<String>("mode")
        .unwrap_or_else(|_| "keep".to_string());
    let mode = mode.trim().to_ascii_lowercase();

    let columns = params::parse_csv_list(&columns_param);
    if columns.is_empty() {
        return Err(BaseError::BadExpression("columns must not be empty".into()));
    }

    let mut lf = table::read_table_lazy(&input.path)?;
    let schema = lf.collect_schema()?;
    let all: Vec<String> = schema.iter_names().map(|s| s.to_string()).collect();

    // Validate requested columns exist.
    let missing: Vec<String> = columns
        .iter()
        .filter(|c| !all.iter().any(|n| n == *c))
        .cloned()
        .collect();
    if !missing.is_empty() {
        return Err(BaseError::SchemaMismatch(format!(
            "unknown columns: {}",
            missing.join(", ")
        )));
    }

    let keep: Vec<String> = match mode.as_str() {
        "keep" => columns,
        "drop" => all
            .iter()
            .filter(|n| !columns.iter().any(|c| c == *n))
            .cloned()
            .collect(),
        other => {
            return Err(BaseError::BadExpression(format!(
                "mode must be 'keep' or 'drop' (got '{other}')"
            )))
        }
    };

    let exprs: Vec<Expr> = keep.iter().map(|c| col(c.as_str())).collect();
    let mut df = lf.select(exprs).collect()?;

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
