//! Block: `base.reduce_join`
//!
//! Join a collection of tables left-to-right on one or more key columns.

use std::path::{Path, PathBuf};

use polars::prelude::*;
use spade::{Args, Outputs, TabularFile, TabularFileCollection};

use crate::common::{params, table, BaseError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<Outputs> {
    let tables: TabularFileCollection = args.input("tables")?;
    let on_param: String = args.param("on")?;
    let how_param: String = args
        .param::<String>("how")
        .unwrap_or_else(|_| "inner".to_string());

    let on = params::parse_csv_list(&on_param);
    if on.is_empty() {
        return Err(BaseError::BadExpression(
            "on must list at least one key column".into(),
        ));
    }

    let join_type = match how_param.trim().to_ascii_lowercase().as_str() {
        "inner" => JoinType::Inner,
        "left" => JoinType::Left,
        "right" => JoinType::Right,
        "outer" | "full" => JoinType::Full,
        other => {
            return Err(BaseError::BadExpression(format!(
                "unknown join type '{other}' (expected one of inner, left, right, outer)"
            )))
        }
    };

    let mut paths = tables.paths.clone();
    paths.sort();
    if paths.is_empty() {
        return Err(BaseError::EmptyCollection);
    }

    let frames: Vec<LazyFrame> = paths
        .iter()
        .map(|p| table::read_table_lazy(p))
        .collect::<Result<Vec<_>>>()?;

    // Validate all frames have the key columns up front.
    let mut missing_details: Vec<String> = Vec::new();
    for (idx, frame) in frames.iter().enumerate() {
        let s = frame.clone().collect_schema()?;
        let names: Vec<String> = s.iter_names().map(|n| n.to_string()).collect();
        for key in &on {
            if !names.iter().any(|n| n == key) {
                missing_details.push(format!("table {idx} is missing key '{key}'"));
            }
        }
    }
    if !missing_details.is_empty() {
        return Err(BaseError::SchemaMismatch(missing_details.join("; ")));
    }

    let key_exprs: Vec<Expr> = on.iter().map(|c| col(c.as_str())).collect();

    let mut iter = frames.into_iter();
    let mut acc = iter.next().expect("non-empty collection checked above");
    for next in iter {
        acc = acc.join(
            next,
            key_exprs.clone(),
            key_exprs.clone(),
            JoinArgs::new(join_type.clone()),
        );
    }
    let mut df = acc.collect()?;

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
