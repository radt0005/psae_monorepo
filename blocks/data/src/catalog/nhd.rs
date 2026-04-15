//! Block: `data.nhd` — USGS National Hydrography Dataset.

use std::path::{Path, PathBuf};

use spade::{Args, Directory};

use crate::catalog::sources::nhd_base_url;
use crate::common::download::{extract_zip_tree, fetch_to};
use crate::common::error::{DataError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<Directory> {
    let huc: String = args
        .param::<String>("huc")
        .map_err(|_| DataError::BadArgument {
            name: "huc".into(),
            reason: "required".into(),
        })?;
    if !huc.chars().all(|c| c.is_ascii_digit()) {
        return Err(DataError::BadArgument {
            name: "huc".into(),
            reason: "HUC must be numeric".into(),
        });
    }
    let resolution: String = args
        .param::<String>("resolution")
        .unwrap_or_else(|_| "high".to_string());
    if resolution != "medium" && resolution != "high" {
        return Err(DataError::BadArgument {
            name: "resolution".into(),
            reason: format!("unknown resolution '{resolution}'"),
        });
    }

    let (size_tag, level) = match huc.len() {
        4 => ("HU4", "HU4"),
        8 => ("HU8", "HU8"),
        other => {
            return Err(DataError::BadArgument {
                name: "huc".into(),
                reason: format!("HUC must be 4 or 8 digits (got {other})"),
            });
        }
    };

    let url = format!(
        "{}/{size_tag}/GPKG/NHD_H_{huc}_{level}_GPKG.zip",
        nhd_base_url()
    );

    let tmp = tempfile::tempdir()?;
    let zip_path = tmp.path().join("nhd.zip");
    fetch_to(&zip_path, &url)?;

    let out_dir = base.join("nhd");
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
