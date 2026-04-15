//! Block: `data.write_collection`
//!
//! Upload each file of a local collection to a remote prefix, keeping
//! basenames. Fails fast on the first error; partial writes are left
//! behind.

use std::path::{Path, PathBuf};

use spade::{Args, FileCollection, JsonFile};

use crate::common::backend::{build_operator, object_key};
use crate::common::download::sha256_of_file;
use crate::common::error::{DataError, Result};
use crate::common::runtime::block_on;
use crate::common::uri::parse as parse_uri;
use crate::read::translate_backend_err;

fn join_prefix(prefix: &str, name: &str) -> String {
    if prefix.is_empty() {
        return name.to_string();
    }
    if prefix.ends_with('/') {
        format!("{prefix}{name}")
    } else {
        format!("{prefix}/{name}")
    }
}

pub(crate) fn handler(args: Args, base: &Path) -> Result<JsonFile> {
    let src: FileCollection = args.input("files")?;
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
    let prefix = object_key(&parsed);

    let mut receipts: Vec<serde_json::Value> = Vec::with_capacity(src.paths.len());
    for path in &src.paths {
        let basename = Path::new(path)
            .file_name()
            .and_then(|s| s.to_str())
            .ok_or_else(|| DataError::BadArgument {
                name: "files".into(),
                reason: format!("bad input filename: {path}"),
            })?;
        let key = join_prefix(&prefix, basename);

        if !overwrite {
            let exists =
                block_on(async { op.exists(&key).await }).map_err(translate_backend_err)?;
            if exists {
                return Err(DataError::BadArgument {
                    name: "uri".into(),
                    reason: format!("destination {key} exists and overwrite=false"),
                });
            }
        }

        let bytes = std::fs::read(path)?;
        let len = bytes.len();
        block_on(async { op.write(&key, bytes).await }).map_err(translate_backend_err)?;
        let sha = sha256_of_file(Path::new(path))?;

        let dest_uri = match parsed.scheme {
            crate::common::uri::Scheme::Fs => format!("file:///{}", key.trim_start_matches('/')),
            _ => format!("{}://{}/{}", parsed.scheme.as_str(), parsed.bucket.as_deref().or(parsed.host.as_deref()).unwrap_or(""), key),
        };

        receipts.push(serde_json::json!({
            "uri": dest_uri,
            "bytes": len,
            "sha256": sha,
        }));
    }

    let out_path = base.join("receipts.json");
    std::fs::write(&out_path, serde_json::to_string_pretty(&receipts)?)?;
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
