//! Block: `base.reduce_stack`
//!
//! Concatenate a collection of tables row-wise (R's `rbind`).

use std::path::{Path, PathBuf};

use polars::prelude::*;
use spade::{Args, Outputs, TabularFile, TabularFileCollection};

use crate::common::{table, BaseError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<Outputs> {
    let tables: TabularFileCollection = args.input("tables")?;
    let strict = args.param::<bool>("strict").unwrap_or(false);

    let mut paths = tables.paths.clone();
    paths.sort();
    if paths.is_empty() {
        return Err(BaseError::EmptyCollection);
    }

    // Load lazy frames so concat_lf / concat_lf_diagonal can optimise.
    let frames: Vec<LazyFrame> = paths
        .iter()
        .map(|p| table::read_table_lazy(p))
        .collect::<Result<Vec<_>>>()?;

    if strict {
        // Verify schemas match exactly.
        let first_schema = frames[0].clone().collect_schema()?;
        for (idx, frame) in frames.iter().enumerate().skip(1) {
            let s = frame.clone().collect_schema()?;
            if s != first_schema {
                return Err(BaseError::SchemaMismatch(format!(
                    "table {idx} schema differs from table 0"
                )));
            }
        }
    }

    let concat_result = if strict {
        concat(&frames, UnionArgs::default())?
    } else {
        concat_lf_diagonal(&frames, UnionArgs::default())?
    };
    let mut df = concat_result.collect()?;

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
