//! Block: `base.reduce_collection`
//!
//! The minimum-viable reducer. It takes a collection input (the outputs of
//! N mapped invocations) and passes the items through unchanged as a
//! single collection output. Despite its triviality it is essential: many
//! pipelines need to "close" a map context without applying any cross-item
//! computation.

use std::path::{Path, PathBuf};

use spade::{Args, FileCollection, Outputs};

use crate::common::Result;

pub(crate) fn handler(args: Args, _base: &Path) -> Result<Outputs> {
    let items: FileCollection = args.input("items")?;
    let mut out = Outputs::new();
    out.add("result", items);
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
