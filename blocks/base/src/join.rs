//! Block: `base.join`
//!
//! Join two tables on one or more shared key columns. The binary counterpart to
//! `base.reduce_join`, which only gathers a map fan-in collection — use this to
//! merge two independently-produced tables (e.g. area estimates with covariates).

use std::path::{Path, PathBuf};

use polars::prelude::*;
use spade::{Args, Outputs, TabularFile};

use crate::common::{params, table, BaseError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<Outputs> {
    let left_input: TabularFile = args.input("left")?;
    let right_input: TabularFile = args.input("right")?;
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

    let left = table::read_table_lazy(&left_input.path)?;
    let right = table::read_table_lazy(&right_input.path)?;

    // Validate the key columns exist in both frames, naming which side is short.
    for (side, frame) in [("left", &left), ("right", &right)] {
        let schema = frame.clone().collect_schema()?;
        let names: Vec<String> = schema.iter_names().map(|n| n.to_string()).collect();
        let missing: Vec<String> = on
            .iter()
            .filter(|k| !names.iter().any(|n| n == *k))
            .cloned()
            .collect();
        if !missing.is_empty() {
            return Err(BaseError::SchemaMismatch(format!(
                "{side} table is missing key column(s): {}",
                missing.join(", ")
            )));
        }
    }

    let key_exprs: Vec<Expr> = on.iter().map(|c| col(c.as_str())).collect();
    let mut df = left
        .join(
            right,
            key_exprs.clone(),
            key_exprs,
            JoinArgs::new(join_type),
        )
        .collect()?;

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
