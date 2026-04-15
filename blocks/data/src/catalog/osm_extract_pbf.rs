//! Block: `data.osm_extract_pbf` — Geofabrik `.osm.pbf` extract.

use std::path::{Path, PathBuf};

use spade::{Args, File as SpadeFile};

use crate::catalog::sources::geofabrik_base_url;
use crate::common::download::fetch_to;
use crate::common::error::{DataError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<SpadeFile> {
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
    let url = format!("{}/{region}-latest.osm.pbf", geofabrik_base_url());
    let basename = format!(
        "{}-latest.osm.pbf",
        region.rsplit('/').next().unwrap_or("extract")
    );
    let out_path = base.join(&basename);
    fetch_to(&out_path, &url)?;
    Ok(SpadeFile::new(out_path.to_string_lossy()))
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
