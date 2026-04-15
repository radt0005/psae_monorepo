//! Block: `data.read_collection`
//!
//! List objects under a URI prefix (optionally filtered by a trailing
//! glob like `*.csv`), fetch each one, and return them as a
//! [`FileCollection`]. `**` is rejected because recursive listing is
//! easy to misuse.

use std::path::{Path, PathBuf};

use spade::{Args, FileCollection};

use crate::common::backend::{build_operator, object_key};
use crate::common::error::{DataError, Result};
use crate::common::runtime::block_on;
use crate::common::uri::parse as parse_uri;
use crate::read::translate_backend_err;

fn split_prefix_and_glob(key: &str) -> Result<(String, Option<String>)> {
    if key.contains("**") {
        return Err(DataError::BadArgument {
            name: "uri".into(),
            reason: "'**' recursive glob is not supported; restructure as a prefix".into(),
        });
    }
    let (dir, tail) = match key.rsplit_once('/') {
        Some((d, t)) => (format!("{d}/"), t.to_string()),
        None => (String::new(), key.to_string()),
    };
    if tail.contains('*') {
        Ok((dir, Some(tail)))
    } else if tail.is_empty() {
        Ok((dir, None))
    } else {
        // Treat trailing no-glob as part of the prefix.
        Ok((format!("{dir}{tail}"), None))
    }
}

fn glob_match(pattern: &str, name: &str) -> bool {
    // Single `*` wildcard in a flat name. Split on `*` and check that
    // each segment appears in order.
    let parts: Vec<&str> = pattern.split('*').collect();
    let mut pos = 0;
    let name_bytes = name.as_bytes();
    for (i, part) in parts.iter().enumerate() {
        if part.is_empty() {
            continue;
        }
        if i == 0 {
            if !name.starts_with(part) {
                return false;
            }
            pos = part.len();
        } else if i == parts.len() - 1 {
            if !name.ends_with(part) {
                return false;
            }
        } else {
            let hay = &name_bytes[pos..];
            let s = std::str::from_utf8(hay).unwrap_or("");
            match s.find(part) {
                Some(j) => pos += j + part.len(),
                None => return false,
            }
        }
    }
    true
}

fn safe_local_name(key: &str) -> String {
    let mut out: String = key.replace('/', "_");
    if out.is_empty() {
        out = "data.bin".into();
    }
    out
}

pub(crate) fn handler(args: Args, base: &Path) -> Result<FileCollection> {
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

    let max_items: u64 = args
        .param::<u64>("max_items")
        .or_else(|_| args.param::<i64>("max_items").map(|n| n.max(0) as u64))
        .unwrap_or(0);

    let parsed = parse_uri(&uri_raw)?;
    let op = build_operator(&parsed)?;
    let key = object_key(&parsed);
    let (prefix, glob) = split_prefix_and_glob(&key)?;

    let entries =
        block_on(async { op.list_with(&prefix).recursive(false).await }).map_err(translate_backend_err)?;

    let mut keys: Vec<String> = entries
        .into_iter()
        .filter_map(|e| {
            let path = e.path().to_string();
            // Skip directory entries.
            if path.ends_with('/') {
                return None;
            }
            let name = Path::new(&path)
                .file_name()
                .map(|s| s.to_string_lossy().to_string())
                .unwrap_or_default();
            if name.is_empty() {
                return None;
            }
            if let Some(g) = &glob {
                if !glob_match(g, &name) {
                    return None;
                }
            }
            Some(path)
        })
        .collect();
    keys.sort();

    if max_items > 0 && keys.len() as u64 > max_items {
        return Err(DataError::BadArgument {
            name: "max_items".into(),
            reason: format!(
                "would fetch {} items; raise max_items or tighten the prefix",
                keys.len()
            ),
        });
    }

    let out_dir: PathBuf = base.join("collection");
    std::fs::create_dir_all(&out_dir)?;

    let mut out_paths = Vec::with_capacity(keys.len());
    for k in keys {
        let bytes = block_on(async { op.read(&k).await }).map_err(translate_backend_err)?;
        let local = safe_local_name(&k);
        let out = out_dir.join(&local);
        std::fs::write(&out, bytes.to_vec())?;
        out_paths.push(out.to_string_lossy().to_string());
    }

    Ok(FileCollection::new(out_paths))
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn glob_matches_simple() {
        assert!(glob_match("*.csv", "a.csv"));
        assert!(glob_match("*.csv", "long-name.csv"));
        assert!(!glob_match("*.csv", "a.txt"));
        assert!(glob_match("prefix_*", "prefix_a"));
        assert!(!glob_match("prefix_*", "other"));
        assert!(glob_match("*", "anything"));
    }

    #[test]
    fn split_no_glob_is_prefix() {
        let (p, g) = split_prefix_and_glob("prefix/").unwrap();
        assert_eq!(p, "prefix/");
        assert!(g.is_none());
    }

    #[test]
    fn split_trailing_glob() {
        let (p, g) = split_prefix_and_glob("prefix/*.csv").unwrap();
        assert_eq!(p, "prefix/");
        assert_eq!(g.as_deref(), Some("*.csv"));
    }

    #[test]
    fn reject_double_star() {
        let err = split_prefix_and_glob("prefix/**/*.csv").unwrap_err();
        assert!(matches!(err, DataError::BadArgument { .. }));
    }
}
