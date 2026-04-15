//! Block: `data.osm_extract_shp` — Geofabrik shapefile extract.

use std::path::{Path, PathBuf};

use spade::{Args, Directory};

use crate::catalog::sources::geofabrik_base_url;
use crate::common::download::{extract_zip_tree, fetch_to};
use crate::common::error::{DataError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<Directory> {
    let region: String = args
        .param::<String>("region")
        .map_err(|_| DataError::BadArgument {
            name: "region".into(),
            reason: "required".into(),
        })?;
    if region.contains("..") || region.starts_with('/') {
        return Err(DataError::BadArgument {
            name: "region".into(),
            reason: "region must be a Geofabrik slug (e.g. north-america/us/oregon)".into(),
        });
    }
    let url = format!("{}/{region}-latest-free.shp.zip", geofabrik_base_url());

    let tmp = tempfile::tempdir()?;
    let zip_path = tmp.path().join("osm.zip");
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
