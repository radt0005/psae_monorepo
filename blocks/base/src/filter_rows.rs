//! Block: `base.filter_rows`
//!
//! Filter rows of a table with a SQL `WHERE`-style predicate expression.
//! Uses `polars-sql` to parse the predicate so users can write standard SQL.

use std::path::{Path, PathBuf};

use polars_sql::SQLContext;
use spade::{Args, Outputs, TabularFile};

use crate::common::{table, BaseError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<Outputs> {
    let input: TabularFile = args.input("table")?;
    let expression: String = args.param("expression")?;
    if expression.trim().is_empty() {
        return Err(BaseError::BadExpression(
            "expression must not be empty".into(),
        ));
    }

    let lf = table::read_table_lazy(&input.path)?;

    let mut ctx = SQLContext::new();
    ctx.register("t", lf);
    let sql = format!("SELECT * FROM t WHERE {expression}");
    let result = ctx
        .execute(&sql)
        .map_err(|e| BaseError::BadExpression(format!("{e}")))?;
    let mut df = result.collect()?;

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
