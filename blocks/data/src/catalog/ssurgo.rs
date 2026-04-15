//! Block: `data.ssurgo` — USDA SSURGO soils data.

use std::path::{Path, PathBuf};

use spade::{Args, Directory};

use crate::catalog::sources::ssurgo_base_url;
use crate::common::download::{extract_zip_tree, fetch_to};
use crate::common::error::{DataError, Result};

pub(crate) fn handler(args: Args, base: &Path) -> Result<Directory> {
    let area = args.param::<String>("area").ok().filter(|s| !s.is_empty());
    let state = args.param::<String>("state").ok().filter(|s| !s.is_empty());

    let filename = match (area.as_deref(), state.as_deref()) {
        (Some(_), Some(_)) => {
            return Err(DataError::BadArgument {
                name: "area/state".into(),
                reason: "exactly one of `area` or `state` must be provided".into(),
            });
        }
        (None, None) => {
            return Err(DataError::BadArgument {
                name: "area/state".into(),
                reason: "exactly one of `area` or `state` must be provided".into(),
            });
        }
        (Some(a), None) => format!("wss_SSA_{}_soildb_US_2003.zip", a.to_uppercase()),
        (None, Some(s)) => format!("wss_gsmsoil_{}.zip", s.to_uppercase()),
    };

    let url = format!("{}/{}", ssurgo_base_url(), filename);

    let tmp = tempfile::tempdir()?;
    let zip_path = tmp.path().join("ssurgo.zip");
    fetch_to(&zip_path, &url)?;

    let out_dir = base.join("ssurgo");
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
