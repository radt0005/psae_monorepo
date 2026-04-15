//! Block: `data.list`
//!
//! Thin wrapper over `Operator::list_with(prefix).recursive(bool)`.

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
    let recursive: bool = args.param::<bool>("recursive").unwrap_or(false);

    let parsed = parse_uri(&uri_raw)?;
    let op = build_operator(&parsed)?;
    let prefix = object_key(&parsed);

    let entries = block_on(async {
        op.list_with(&prefix)
            .recursive(recursive)
            .metakey(opendal::Metakey::Complete)
            .await
    })
    .map_err(translate_backend_err)?;

    let mut rows: Vec<serde_json::Value> = entries
        .into_iter()
        .filter_map(|e| {
            let key = e.path().to_string();
            if key.ends_with('/') {
                // Skip pure directory markers but keep trailing ones with content elsewhere.
                return None;
            }
            let meta = e.metadata();
            let size = meta.content_length();
            let last_modified = meta.last_modified().map(|t| t.to_rfc3339());
            let etag = meta.etag().map(|s| s.to_string());
            Some(serde_json::json!({
                "key": key,
                "size": size,
                "last_modified": last_modified,
                "etag": etag,
            }))
        })
        .collect();
    rows.sort_by(|a, b| {
        a.get("key").and_then(|v| v.as_str()).unwrap_or("").cmp(
            b.get("key").and_then(|v| v.as_str()).unwrap_or(""),
        )
    });

    let out_path = base.join("listing.json");
    std::fs::write(&out_path, serde_json::to_string_pretty(&rows)?)?;
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
