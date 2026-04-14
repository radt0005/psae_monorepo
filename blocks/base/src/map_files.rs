//! Block: `base.map_files`
//!
//! Enumerates a file collection and writes an expansion manifest so the
//! scheduler can fan the downstream pipeline out over each file.
//!
//! This block bypasses the standard `IntoOutput` pipeline because the runtime
//! does not model `type: expansion` as a first-class type; we write the
//! `expansion.yaml` file directly.

use std::path::{Path, PathBuf};

use spade::{Args, FileCollection};

use crate::common::expansion::{self, ExpansionItem};
use crate::common::Result;

pub(crate) fn handler(args: Args, base: &Path) -> Result<()> {
    let source: FileCollection = args.input("source")?;
    let mut paths = source.paths.clone();
    paths.sort();

    let items: Vec<ExpansionItem> = paths
        .iter()
        .map(|full_path| {
            let p = Path::new(full_path);
            let key = p
                .file_stem()
                .and_then(|s| s.to_str())
                .unwrap_or("item")
                .to_string();
            // Relative path from the working directory to the input file.
            let rel = path_relative_to(full_path, base);
            ExpansionItem { path: rel, key }
        })
        .collect();

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

fn path_relative_to(full_path: &str, base: &Path) -> String {
    let full = Path::new(full_path);
    match full.strip_prefix(base) {
        Ok(rel) => rel.to_string_lossy().to_string(),
        Err(_) => full_path.to_string(),
    }
}
