//! Block: `base.pivot`
//!
//! Reshape a table between long and wide form. "wider" spreads the distinct
//! values of `names_from` into new columns filled from `values_from`; "longer"
//! gathers the `values_from` columns into a `variable`/`value` pair keyed by
//! `id_columns`.

use std::path::{Path, PathBuf};

use polars::prelude::*;
use polars_ops::pivot::{pivot, PivotAgg, UnpivotDF};
use spade::{Args, Outputs, TabularFile};

use crate::common::{column_names, params, table, BaseError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<Outputs> {
    let input: TabularFile = args.input("table")?;
    let direction: String = args
        .param::<String>("direction")
        .unwrap_or_else(|_| "wider".to_string())
        .trim()
        .to_ascii_lowercase();
    let id_columns: String = args.param::<String>("id_columns").unwrap_or_default();
    let names_from: String = args.param::<String>("names_from").unwrap_or_default();
    let values_from: String = args.param::<String>("values_from").unwrap_or_default();
    let fill: String = args.param::<String>("fill").unwrap_or_default();

    let df = table::read_table(&input.path)?;
    let all = column_names(&df);
    let idx = params::parse_csv_list(&id_columns);

    let check_exist = |cols: &[String]| -> Result<()> {
        let missing: Vec<String> = cols
            .iter()
            .filter(|c| !all.iter().any(|n| n == *c))
            .cloned()
            .collect();
        if missing.is_empty() {
            Ok(())
        } else {
            Err(BaseError::SchemaMismatch(format!(
                "unknown columns: {}",
                missing.join(", ")
            )))
        }
    };
    check_exist(&idx)?;

    let mut result = match direction.as_str() {
        "wider" => {
            let nf = params::parse_csv_list(&names_from);
            let vf = params::parse_csv_list(&values_from);
            if nf.is_empty() {
                return Err(BaseError::BadExpression(
                    "names_from is required for direction=wider".into(),
                ));
            }
            if vf.is_empty() {
                return Err(BaseError::BadExpression(
                    "values_from is required for direction=wider".into(),
                ));
            }
            check_exist(&nf)?;
            check_exist(&vf)?;

            let mut r = pivot(
                &df,
                nf,
                Some(idx.clone()),
                Some(vf),
                false,
                Some(PivotAgg::First),
                None,
            )?;

            if !fill.trim().is_empty() {
                let fv: f64 = fill.trim().parse().map_err(|_| {
                    BaseError::BadExpression(format!("fill must be numeric (got '{fill}')"))
                })?;
                let cols = column_names(&r);
                let exprs: Vec<Expr> = cols
                    .iter()
                    .map(|c| {
                        if idx.iter().any(|i| i == c) {
                            col(c.as_str())
                        } else {
                            col(c.as_str()).fill_null(lit(fv))
                        }
                    })
                    .collect();
                r = r.lazy().select(exprs).collect()?;
            }
            r
        }
        "longer" => {
            if idx.is_empty() {
                return Err(BaseError::BadExpression(
                    "id_columns is required for direction=longer".into(),
                ));
            }
            let mut on = params::parse_csv_list(&values_from);
            if on.is_empty() {
                on = all
                    .iter()
                    .filter(|c| !idx.iter().any(|i| i == *c))
                    .cloned()
                    .collect();
            }
            check_exist(&on)?;
            df.unpivot(on, idx.clone())?
        }
        other => {
            return Err(BaseError::BadExpression(format!(
                "direction must be 'wider' or 'longer' (got '{other}')"
            )))
        }
    };

    let out_path: PathBuf = base.join("result.parquet");
    table::write_parquet(&mut result, out_path.to_str().expect("utf-8 path"))?;

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
