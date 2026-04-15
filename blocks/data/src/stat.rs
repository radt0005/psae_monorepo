//! Block: `data.stat`
//!
//! Fetch metadata for a single object.

use std::path::{Path, PathBuf};

use spade::{Args, JsonFile};

use crate::common::backend::{build_operator, object_key};
use crate::common::error::{DataError, Result};
use crate::common::runtime::block_on;
use crate::common::uri::parse as parse_uri;
use crate::read::translate_backend_err;

pub(crate) fn handler(args: Args, base: &Path) -> Result<JsonFile> {
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

    let parsed = parse_uri(&uri_raw)?;
    let op = build_operator(&parsed)?;
    let key = object_key(&parsed);

    let meta = block_on(async { op.stat(&key).await }).map_err(translate_backend_err)?;

    let body = serde_json::json!({
        "key": key,
        "size": meta.content_length(),
        "last_modified": meta.last_modified().map(|t| t.to_rfc3339()),
        "etag": meta.etag(),
        "content_type": meta.content_type(),
    });

    let out_path = base.join("metadata.json");
    std::fs::write(&out_path, serde_json::to_string_pretty(&body)?)?;
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
