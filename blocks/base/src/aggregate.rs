//! Block: `base.aggregate`
//!
//! Compute aggregations over a table, optionally grouped.
//! Supported functions: mean, median, mode, sum, min, max, count,
//! count_distinct, std, var, percentile.

use std::path::{Path, PathBuf};

use polars::prelude::*;
use serde::Deserialize;
use spade::{Args, Outputs, TabularFile};

use crate::common::{params, table, BaseError, Result};

#[derive(Debug, Clone, Deserialize)]
pub struct AggregationSpec {
    pub column: String,
    pub function: String,
    #[serde(default)]
    pub p: Option<f64>,
    #[serde(rename = "as", default)]
    pub alias: Option<String>,
}

fn default_alias(spec: &AggregationSpec) -> String {
    match (spec.function.as_str(), spec.p) {
        ("percentile", Some(p)) => format!("{}_p{}", spec.column, (p * 100.0) as i64),
        (func, _) => format!("{}_{}", spec.column, func),
    }
}

fn build_expr(spec: &AggregationSpec) -> Result<Expr> {
    let c = col(spec.column.as_str());
    let expr = match spec.function.as_str() {
        "mean" => c.mean(),
        "median" => c.median(),
        "mode" => c.mode().first(),
        "sum" => c.sum(),
        "min" => c.min(),
        "max" => c.max(),
        "count" => c.count(),
        "count_distinct" => c.n_unique(),
        "std" => c.std(1),
        "var" => c.var(1),
        "percentile" => {
            let p = spec.p.ok_or_else(|| {
                BaseError::InvalidAggregation(format!(
                    "percentile aggregation on '{}' requires 'p'",
                    spec.column
                ))
            })?;
            if !(0.0..=1.0).contains(&p) {
                return Err(BaseError::InvalidAggregation(format!(
                    "percentile 'p' must be in [0, 1] (got {p})"
                )));
            }
            c.quantile(lit(p), QuantileMethod::Linear)
        }
        other => {
            return Err(BaseError::InvalidAggregation(format!(
                "unknown aggregation function: '{other}'"
            )))
        }
    };
    let alias = spec.alias.clone().unwrap_or_else(|| default_alias(spec));
    Ok(expr.alias(alias.as_str()))
}

/// Run aggregations, optionally grouped, against a LazyFrame.
pub(crate) fn run_aggregations(
    lf: LazyFrame,
    group_cols: &[String],
    specs: &[AggregationSpec],
) -> Result<DataFrame> {
    if specs.is_empty() {
        return Err(BaseError::InvalidAggregation(
            "at least one aggregation is required".into(),
        ));
    }
    let exprs: Vec<Expr> = specs.iter().map(build_expr).collect::<Result<Vec<_>>>()?;

    if group_cols.is_empty() {
        Ok(lf.select(exprs).collect()?)
    } else {
        let by: Vec<Expr> = group_cols.iter().map(|c| col(c.as_str())).collect();
        let df = lf
            .group_by(by.clone())
            .agg(exprs)
            .sort(group_cols, SortMultipleOptions::default())
            .collect()?;
        Ok(df)
    }
}

pub(crate) fn handler(args: Args, base: &Path) -> Result<Outputs> {
    let input: TabularFile = args.input("table")?;
    let aggregations: String = args.param("aggregations")?;
    let group_by = args
        .param::<String>("group_by")
        .unwrap_or_else(|_| String::new());

    let specs: Vec<AggregationSpec> = params::parse_json_list(&aggregations)?;
    let group_cols = params::parse_csv_list(&group_by);

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
