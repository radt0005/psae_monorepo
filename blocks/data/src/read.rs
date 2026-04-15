//! Block: `data.read`
//!
//! Fetch a single object from any supported backend. The backend is
//! inferred from the URI scheme. The handler streams bytes into a local
//! file in the working directory and returns a [`spade::File`] — no
//! format is baked into the output manifest because the format is
//! resolved at runtime.

use std::path::{Path, PathBuf};

use spade::{Args, File as SpadeFile};

use crate::common::backend::{build_operator, object_key};
use crate::common::error::{DataError, Result};
use crate::common::runtime::block_on;
use crate::common::uri::parse as parse_uri;

fn sanitize_filename(raw: &str) -> String {
    let basename = std::path::Path::new(raw)
        .file_name()
        .and_then(|s| s.to_str())
        .unwrap_or("");
    // Reject anything that would traverse up or sneak in a separator.
    let clean: String = basename
        .chars()
        .filter(|c| !matches!(c, '/' | '\\'))
        .collect();
    if clean.is_empty() || clean == "." || clean == ".." {
        "data.bin".to_string()
    } else {
        clean
    }
}

pub(crate) fn handler(args: Args, base: &Path) -> Result<SpadeFile> {
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

    let format = args.param::<String>("format").ok().filter(|s| !s.is_empty());

    let parsed = parse_uri(&uri_raw)?;
    let op = build_operator(&parsed)?;
    let key = object_key(&parsed);

    let filename = sanitize_filename(&parsed.path);
    let out_path: PathBuf = base.join(&filename);

    let bytes = block_on(async { op.read(&key).await }).map_err(translate_backend_err)?;
    let bytes_vec: Vec<u8> = bytes.to_vec();
    let len = bytes_vec.len();
    std::fs::write(&out_path, &bytes_vec)?;

    // Emit an invocation sidecar with the resolved URI and format.
    if let Some(fmt) = format.as_deref() {
        let sidecar = base.join("invocation.json");
        let body = serde_json::json!({
            "uri": uri_raw,
            "format": fmt,
            "bytes": len,
        });
        std::fs::write(&sidecar, serde_json::to_string_pretty(&body)?)?;
    }

    Ok(SpadeFile::new(out_path.to_string_lossy()))
}

/// Map an OpenDAL error into a more specific [`DataError`] variant.
pub(crate) fn translate_backend_err(err: opendal::Error) -> DataError {
    match err.kind() {
        opendal::ErrorKind::NotFound => DataError::NotFound(err.to_string()),
        opendal::ErrorKind::PermissionDenied => DataError::Unauthorized(err.to_string()),
        _ => DataError::from(err),
    }
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
