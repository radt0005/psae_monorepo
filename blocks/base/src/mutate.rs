//! Block: `base.mutate`
//!
//! Add or replace columns computed from SQL scalar expressions. Each spec's
//! expression is parsed by `polars-sql` and assigned to a named column via
//! `with_columns`, which replaces a column of the same name or appends a new
//! one — matching a dplyr-style `mutate`.

use std::path::{Path, PathBuf};

use polars_sql::sql_expr;
use serde::Deserialize;
use spade::{Args, Outputs, TabularFile};

use crate::common::{params, table, BaseError, Result};

#[derive(Debug, Deserialize)]
struct MutateSpec {
    name: String,
    expr: String,
}

pub(crate) fn handler(args: Args, base: &Path) -> Result<Outputs> {
    let input: TabularFile = args.input("table")?;
    let expressions: String = args.param("expressions")?;

    let specs: Vec<MutateSpec> = params::parse_json_list(&expressions)?;
    if specs.is_empty() {
        return Err(BaseError::BadExpression(
            "expressions must be a non-empty JSON list".into(),
        ));
    }

    let lf = table::read_table_lazy(&input.path)?;

    let mut exprs = Vec::with_capacity(specs.len());
    for spec in &specs {
        if spec.name.trim().is_empty() {
            return Err(BaseError::BadExpression(
                "each expression needs a non-empty 'name'".into(),
            ));
        }
        let expr = sql_expr(&spec.expr)
            .map_err(|e| BaseError::BadExpression(format!("{}: {e}", spec.name)))?;
        exprs.push(expr.alias(spec.name.as_str()));
    }

    let mut df = lf.with_columns(exprs).collect()?;

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
