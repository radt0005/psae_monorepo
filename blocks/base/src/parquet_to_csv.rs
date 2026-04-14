//! Block: `base.parquet_to_csv`
//!
//! Convert a Parquet file to CSV.

use std::path::{Path, PathBuf};

use spade::{Args, Outputs, TabularFile};

use crate::common::{table, BaseError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<Outputs> {
    let input: TabularFile = args.input("table")?;
    let delimiter = param_delimiter(&args, "delimiter", b',')?;
    let include_header = args.param::<bool>("include_header").unwrap_or(true);

    let mut df = table::read_table(&input.path)?;
    let out_path: PathBuf = base.join("result.csv");
    table::write_csv(
        &mut df,
        out_path.to_str().expect("utf-8 path"),
        delimiter,
        include_header,
    )?;

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
