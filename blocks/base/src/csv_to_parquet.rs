//! Block: `base.csv_to_parquet`
//!
//! Convert a CSV file to Parquet. CSV dialect is configurable via `delimiter`
//! and `has_header` params.

use std::path::{Path, PathBuf};

use polars::prelude::*;
use spade::{Args, Outputs, TabularFile};

use crate::common::{table, BaseError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<Outputs> {
    let table: TabularFile = args.input("table")?;
    let delimiter = param_delimiter(&args, "delimiter", b',')?;
    let has_header = args.param::<bool>("has_header").unwrap_or(true);

    let lf = LazyCsvReader::new(&table.path)
        .with_has_header(has_header)
        .with_separator(delimiter)
        .with_infer_schema_length(Some(1024))
        .finish()?;
    let mut df = lf.collect()?;

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

fn param_delimiter(args: &Args, name: &str, default: u8) -> Result<u8> {
    if !args.has_param(name) {
        return Ok(default);
    }
    let s: String = args.param(name)?;
    let bytes = s.as_bytes();
    if bytes.len() != 1 {
        return Err(BaseError::BadExpression(format!(
            "{name} must be a single ASCII character"
        )));
    }
    Ok(bytes[0])
}
