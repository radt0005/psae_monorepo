//! Access to secrets injected into a block by the Spade runtime.
//!
//! Secrets are delivered as the `SPADE_SECRETS` environment variable — a JSON
//! object mapping the block's logical secret names to their values — by the
//! worker (cloud) or CLI (local). This module parses that blob, serves values
//! through [`get_secret`], and scrubs the variable from the environment so it is
//! not inherited by any subprocess the block spawns. See `spec/secrets.md` §4.

use std::collections::HashMap;
use std::sync::OnceLock;

use crate::error::{Result, SpadeError};

static SECRETS: OnceLock<HashMap<String, String>> = OnceLock::new();

fn parse_secrets(raw: &str) -> HashMap<String, String> {
    if raw.is_empty() {
        return HashMap::new();
    }
    serde_json::from_str(raw).unwrap_or_default()
}

/// Load and cache `SPADE_SECRETS`, removing it from the environment on first
/// read so it is not inherited by subprocesses the block spawns. Idempotent.
pub(crate) fn load_secrets() -> &'static HashMap<String, String> {
    SECRETS.get_or_init(|| {
        let parsed = match std::env::var("SPADE_SECRETS") {
            Ok(raw) => parse_secrets(&raw),
            Err(_) => HashMap::new(),
        };
        // SAFETY: invoked during single-threaded block startup (from `run`)
        // before the handler spawns any threads. Removing the variable keeps
        // secret values out of child process environments.
        unsafe {
            std::env::remove_var("SPADE_SECRETS");
        }
        parsed
    })
}

fn lookup(secrets: &HashMap<String, String>, name: &str) -> Result<String> {
    secrets
        .get(name)
        .cloned()
        .ok_or_else(|| SpadeError::SecretNotFound {
            name: name.to_string(),
        })
}

/// Return the secret bound to a logical `name` for this block.
///
/// The mapping from logical name to a stored secret is declared in the pipeline
/// (see `spec/secrets.md` §3.2); the value is injected by the worker (cloud) or
/// CLI (local). Returns [`SpadeError::SecretNotFound`] if the name was not
/// provided — a declared-but-unresolved secret is a real error, not empty.
pub fn get_secret(name: &str) -> Result<String> {
    lookup(load_secrets(), name)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_json_blob() {
        let m = parse_secrets(r#"{"db":"postgres://x","api":"k"}"#);
        assert_eq!(m.get("db").map(String::as_str), Some("postgres://x"));
        assert_eq!(m.get("api").map(String::as_str), Some("k"));
    }

    #[test]
    fn empty_blob_is_empty() {
        assert!(parse_secrets("").is_empty());
    }

    #[test]
    fn lookup_hit_and_miss() {
        let m = parse_secrets(r#"{"db":"x"}"#);
        assert_eq!(lookup(&m, "db").unwrap(), "x");
        assert!(matches!(
            lookup(&m, "nope"),
            Err(SpadeError::SecretNotFound { .. })
        ));
    }
}
