//! Block: `data.naturalearth_raster` — Natural Earth raster datasets.

use std::path::{Path, PathBuf};

use spade::{Args, RasterFile};

use crate::catalog::sources::naturalearth_base_url;
use crate::common::download::{extract_zip_tree, fetch_to};
use crate::common::error::{DataError, Result};

fn validate_scale(scale: &str) -> Result<&'static str> {
    Ok(match scale {
        "10m" => "10m",
        "50m" => "50m",
        other => {
            return Err(DataError::BadArgument {
                name: "scale".into(),
                reason: format!(
                    "unknown scale '{other}' (Natural Earth rasters only ship at 10m and 50m)"
                ),
            })
        }
    })
}

pub(crate) fn handler(args: Args, base: &Path) -> Result<RasterFile> {
    let scale = validate_scale(&args.param::<String>("scale").map_err(|_| {
        DataError::BadArgument {
            name: "scale".into(),
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
        "{}/{}/raster/{}.zip",
        naturalearth_base_url(),
        scale,
        theme
    );

    let tmp = tempfile::tempdir()?;
    let zip_path = tmp.path().join("ne.zip");
    fetch_to(&zip_path, &url)?;

    let out_dir = base.join("raster");
    let files = extract_zip_tree(&zip_path, &out_dir)?;

    // Natural Earth raster archives ship one main image (TIF or similar).
    // Pick the largest .tif/.tiff or fall back to the first file.
    let chosen = files
        .iter()
        .filter(|p| {
            matches!(
                p.extension().and_then(|s| s.to_str()),
                Some("tif") | Some("tiff") | Some("TIF") | Some("TIFF")
            )
        })
        .max_by_key(|p| std::fs::metadata(p).map(|m| m.len()).unwrap_or(0))
        .cloned()
        .or_else(|| files.first().cloned())
        .ok_or_else(|| DataError::BadArgument {
            name: "theme".into(),
            reason: "archive contained no files".into(),
        })?;
    Ok(RasterFile::new(chosen.to_string_lossy()))
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
