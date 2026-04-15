//! Block: `data.write`
//!
//! Upload a single local file to any supported backend. Refuses to
//! overwrite existing objects unless `overwrite=true`. Emits a
//! JSON receipt describing the upload (destination URI, byte count,
//! sha256 of the uploaded content).

use std::path::{Path, PathBuf};

use spade::{Args, JsonFile};

use crate::common::backend::{build_operator, object_key};
use crate::common::download::sha256_of_file;
use crate::common::error::{DataError, Result};
use crate::common::runtime::block_on;
use crate::common::uri::parse as parse_uri;
use crate::read::translate_backend_err;

pub(crate) fn handler(args: Args, base: &Path) -> Result<JsonFile> {
    let src: spade::File = args.input("file")?;
    let uri_raw: String = args.param::<String>("uri").map_err(|_| DataError::BadArgument {
        name: "uri".into(),
        reason: "required".into(),
    })?;
    if uri_raw.trim().is_empty() {
        return Err(DataError::BadArgument {
            name: "uri".into(),
            reason: "empty".into(),
        });
    }
    let overwrite: bool = args.param::<bool>("overwrite").unwrap_or(false);

    let parsed = parse_uri(&uri_raw)?;
    let op = build_operator(&parsed)?;
    let key = object_key(&parsed);

    if !overwrite {
        let exists = block_on(async { op.exists(&key).await }).map_err(translate_backend_err)?;
        if exists {
            return Err(DataError::BadArgument {
                name: "uri".into(),
                reason: "destination exists and overwrite=false".into(),
            });
        }
    }

    let bytes = std::fs::read(&src.path)?;
    let len = bytes.len();
    block_on(async { op.write(&key, bytes).await }).map_err(translate_backend_err)?;

    let sha = sha256_of_file(Path::new(&src.path))?;

    let receipt = serde_json::json!({
        "uri": uri_raw,
        "bytes": len,
        "sha256": sha,
    });

    let out_path = base.join("receipt.json");
    std::fs::write(&out_path, serde_json::to_string_pretty(&receipt)?)?;
    Ok(JsonFile::new(out_path.to_string_lossy()))
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
