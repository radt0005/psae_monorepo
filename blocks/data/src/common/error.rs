//! Collection-local error type.
//!
//! Every handler in this collection bubbles its failures up through
//! [`DataError`]. The variants cover the usual shape of data-import
//! problems: bad URIs, network failures, missing credentials, unexpected
//! formats, and the plumbing errors that OpenDAL/reqwest raise.

use thiserror::Error;

/// All failures produced by blocks in this collection.
#[derive(Debug, Error)]
pub enum DataError {
    #[error("unsupported URI scheme: '{0}'")]
    UnsupportedScheme(String),

    #[error("bad URI '{uri}': {reason}")]
    BadUri { uri: String, reason: String },

    #[error("network error fetching '{uri}': {source}")]
    Network {
        uri: String,
        #[source]
        source: Box<dyn std::error::Error + Send + Sync>,
    },

    #[error("not found: {0}")]
    NotFound(String),

    #[error("unauthorized: {0}")]
    Unauthorized(String),

    #[error("backend error: {0}")]
    Backend(#[from] Box<opendal::Error>),

    #[error("HTTP error: {0}")]
    Http(#[from] Box<reqwest::Error>),

    #[error("I/O error: {0}")]
    Io(#[from] std::io::Error),

    #[error("YAML error: {0}")]
    Yaml(#[from] serde_yaml::Error),

    #[error("JSON error: {0}")]
    Json(#[from] serde_json::Error),

    #[error("zip error: {0}")]
    Zip(#[from] zip::result::ZipError),

    #[error("checksum mismatch for '{uri}': expected {expected}, got {actual}")]
    ChecksumMismatch {
        uri: String,
        expected: String,
        actual: String,
    },

    #[error("unsupported dataset: {0}")]
    UnsupportedDataset(String),

    #[error("bad argument '{name}': {reason}")]
    BadArgument { name: String, reason: String },

    #[error("spade runtime error: {0}")]
    Spade(#[from] spade::SpadeError),
}

/// Short alias used throughout the collection.
pub type Result<T> = std::result::Result<T, DataError>;

impl From<opendal::Error> for DataError {
    fn from(e: opendal::Error) -> Self {
        DataError::Backend(Box::new(e))
    }
}

impl From<reqwest::Error> for DataError {
    fn from(e: reqwest::Error) -> Self {
        DataError::Http(Box::new(e))
    }
}
