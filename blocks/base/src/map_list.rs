//! Block: `base.map_list`
//!
//! Fan out over a literal list of scalar values (strings or numbers).
//! Each value is materialised as a small JSON file and recorded in an
//! expansion manifest.

use std::path::{Path, PathBuf};

use spade::Args;

use crate::common::expansion;
use crate::common::{BaseError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<()> {
    let values_param: String = args.param("values")?;
    let values: Vec<serde_json::Value> = serde_json::from_str(&values_param)?;

    validate_scalars(&values)?;
    if values.is_empty() {
        return Err(BaseError::EmptyCollection);
    }

    let items = expansion::materialise_scalar_items(base, &values)?;
    expansion::write_manifest(base, items)?;
    Ok(())
}

pub fn entry() {
    let base = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
    spade::run(move |args| handler(args, &base).map_err(Into::into));
}

pub fn run_in(base: &Path) -> Result<()> {
    let args = spade::scanning::build_args_from(base)?;
    handler(args, base)?;
    Ok(())
}

fn validate_scalars(values: &[serde_json::Value]) -> Result<()> {
    for (i, v) in values.iter().enumerate() {
        match v {
            serde_json::Value::String(_)
            | serde_json::Value::Number(_)
            | serde_json::Value::Bool(_)
            | serde_json::Value::Null => {}
            _ => {
                return Err(BaseError::BadExpression(format!(
                    "values[{i}] must be a scalar (string, number, boolean, or null)"
                )))
            }
        }
    }
    Ok(())
}
