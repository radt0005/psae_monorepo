//! Block: `data.naturalearth_vector` — Natural Earth vector datasets.

use std::path::{Path, PathBuf};

use spade::{Args, Directory};

use crate::catalog::sources::naturalearth_base_url;
use crate::common::download::{extract_zip_tree, fetch_to};
use crate::common::error::{DataError, Result};

fn validate_scale(scale: &str) -> Result<&'static str> {
    Ok(match scale {
        "10m" => "10m",
        "50m" => "50m",
        "110m" => "110m",
        other => {
            return Err(DataError::BadArgument {
                name: "scale".into(),
                reason: format!("unknown scale '{other}' (expected 10m, 50m, 110m)"),
            })
        }
    })
}

fn validate_category(category: &str) -> Result<&'static str> {
    Ok(match category {
        "cultural" => "cultural",
        "physical" => "physical",
        other => {
            return Err(DataError::BadArgument {
                name: "category".into(),
                reason: format!("unknown category '{other}'"),
            })
        }
    })
}

pub(crate) fn handler(args: Args, base: &Path) -> Result<Directory> {
    let scale = validate_scale(&args.param::<String>("scale").map_err(|_| {
        DataError::BadArgument {
            name: "scale".into(),
            reason: "required".into(),
        }
    })?)?;
    let category = validate_category(&args.param::<String>("category").map_err(|_| {
        DataError::BadArgument {
            name: "category".into(),
            reason: "required".into(),
        }
    })?)?;
    let theme: String = args
        .param::<String>("theme")
        .map_err(|_| DataError::BadArgument {
            name: "theme".into(),
            reason: "required".into(),
        })?;

    let url = format!(
        "{}/{}/{}/ne_{}_{}.zip",
        naturalearth_base_url(),
        scale,
        category,
        scale,
        theme
    );

    let tmp = tempfile::tempdir()?;
    let zip_path = tmp.path().join("ne.zip");
    fetch_to(&zip_path, &url)?;

    let out_dir = base.join("shapefile");
    extract_zip_tree(&zip_path, &out_dir)?;
    Ok(Directory::new(out_dir.to_string_lossy()))
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
