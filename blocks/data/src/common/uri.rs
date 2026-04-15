//! URI parsing and scheme dispatch.
//!
//! The "one read block" story relies on turning a user-provided URI into a
//! normalised [`ParsedUri`] that backend-construction can dispatch on.
//! Bare absolute paths (no scheme) are accepted as a convenience and
//! treated as `file://<path>`.

use std::collections::HashMap;

use crate::common::error::{DataError, Result};

/// Supported backend schemes.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Scheme {
    S3,
    Gcs,
    AzBlob,
    Http,
    Https,
    Fs,
    Sftp,
    Ftp,
    Webdav,
    GoogleDrive,
    OneDrive,
    Dropbox,
    /// Memory backend; test-only.
    Memory,
}

impl Scheme {
    /// Parse a scheme name (case-insensitive, without the trailing `://`).
    pub fn parse(s: &str) -> Result<Self> {
        let lower = s.to_ascii_lowercase();
        Ok(match lower.as_str() {
            "s3" => Scheme::S3,
            "gs" | "gcs" => Scheme::Gcs,
            "azblob" | "az" => Scheme::AzBlob,
            "http" => Scheme::Http,
            "https" => Scheme::Https,
            "file" => Scheme::Fs,
            "sftp" => Scheme::Sftp,
            "ftp" => Scheme::Ftp,
            "webdav" => Scheme::Webdav,
            "gdrive" | "googledrive" => Scheme::GoogleDrive,
            "onedrive" => Scheme::OneDrive,
            "dropbox" => Scheme::Dropbox,
            "memory" => Scheme::Memory,
            other => return Err(DataError::UnsupportedScheme(other.to_string())),
        })
    }

    /// Human-readable scheme name (matches the canonical form used in URIs).
    pub fn as_str(&self) -> &'static str {
        match self {
            Scheme::S3 => "s3",
            Scheme::Gcs => "gs",
            Scheme::AzBlob => "azblob",
            Scheme::Http => "http",
            Scheme::Https => "https",
            Scheme::Fs => "file",
            Scheme::Sftp => "sftp",
            Scheme::Ftp => "ftp",
            Scheme::Webdav => "webdav",
            Scheme::GoogleDrive => "gdrive",
            Scheme::OneDrive => "onedrive",
            Scheme::Dropbox => "dropbox",
            Scheme::Memory => "memory",
        }
    }
}

/// Normalised URI suitable for feeding an OpenDAL operator.
#[derive(Debug, Clone)]
pub struct ParsedUri {
    pub scheme: Scheme,
    pub host: Option<String>,
    pub bucket: Option<String>,
    pub path: String,
    pub query: HashMap<String, String>,
    /// The original input, preserved for diagnostics and HTTP fetches.
    pub original: String,
}

impl ParsedUri {
    /// Return the URI in the canonical `scheme://host/path` form suitable
    /// for reconstruction.
    pub fn original(&self) -> &str {
        &self.original
    }
}

/// Parse a URI, accepting bare absolute paths as a convenience.
pub fn parse(uri: &str) -> Result<ParsedUri> {
    if uri.is_empty() {
        return Err(DataError::BadArgument {
            name: "uri".to_string(),
            reason: "empty".to_string(),
        });
    }

    // Bare absolute path → file://
    if uri.starts_with('/') {
        return Ok(ParsedUri {
            scheme: Scheme::Fs,
            host: None,
            bucket: None,
            path: uri.to_string(),
            query: HashMap::new(),
            original: format!("file://{uri}"),
        });
    }

    // Detect the scheme manually. `url::Url::parse` rejects some schemes
    // we care about (e.g. `memory://`) in strict mode on older versions,
    // and we want to keep error messages under our control.
    let scheme_end = uri.find("://").ok_or_else(|| DataError::BadUri {
        uri: uri.to_string(),
        reason: "missing scheme and not an absolute path".to_string(),
    })?;
    let scheme_str = &uri[..scheme_end];
    let scheme = Scheme::parse(scheme_str)?;

    // Route through url::Url once we know the scheme is one we accept.
    // Memory URIs (`memory:///key`) are also handled but `url` treats the
    // authority as empty; that is fine.
    let parsed = url::Url::parse(uri).map_err(|e| DataError::BadUri {
        uri: uri.to_string(),
        reason: format!("malformed URL: {e}"),
    })?;

    let host = parsed.host_str().map(|s| s.to_string());
    let mut path = parsed.path().to_string();
    // url::Url always leaves a leading slash on the path for hierarchical
    // URIs. For S3/GCS/Azure we strip it since the bucket is the host.
    let bucket = match scheme {
        Scheme::S3 | Scheme::Gcs | Scheme::AzBlob => {
            if path.starts_with('/') {
                path.remove(0);
            }
            host.clone()
        }
        _ => None,
    };

    let query: HashMap<String, String> = parsed
        .query_pairs()
        .map(|(k, v)| (k.into_owned(), v.into_owned()))
        .collect();

    Ok(ParsedUri {
        scheme,
        host,
        bucket,
        path,
        query,
        original: uri.to_string(),
    })
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_file_scheme() {
        let p = parse("file:///tmp/hello.txt").unwrap();
        assert_eq!(p.scheme, Scheme::Fs);
        assert_eq!(p.path, "/tmp/hello.txt");
    }

    #[test]
    fn parse_bare_path() {
        let p = parse("/tmp/hello.txt").unwrap();
        assert_eq!(p.scheme, Scheme::Fs);
        assert_eq!(p.path, "/tmp/hello.txt");
    }

    #[test]
    fn parse_s3() {
        let p = parse("s3://mybucket/path/to/key").unwrap();
        assert_eq!(p.scheme, Scheme::S3);
        assert_eq!(p.bucket.as_deref(), Some("mybucket"));
        assert_eq!(p.path, "path/to/key");
    }

    #[test]
    fn parse_gcs() {
        let p = parse("gs://bucket/object.txt").unwrap();
        assert_eq!(p.scheme, Scheme::Gcs);
        assert_eq!(p.bucket.as_deref(), Some("bucket"));
    }

    #[test]
    fn parse_azblob() {
        let p = parse("azblob://container/file.bin").unwrap();
        assert_eq!(p.scheme, Scheme::AzBlob);
        assert_eq!(p.bucket.as_deref(), Some("container"));
    }

    #[test]
    fn parse_https() {
        let p = parse("https://example.com/data.csv").unwrap();
        assert_eq!(p.scheme, Scheme::Https);
        assert_eq!(p.host.as_deref(), Some("example.com"));
        assert_eq!(p.path, "/data.csv");
    }

    #[test]
    fn parse_memory() {
        let p = parse("memory:///test/key").unwrap();
        assert_eq!(p.scheme, Scheme::Memory);
    }

    #[test]
    fn reject_unsupported() {
        let err = parse("ipfs://Qm.../file").unwrap_err();
        assert!(matches!(err, DataError::UnsupportedScheme(_)));
    }

    #[test]
    fn reject_relative() {
        let err = parse("relative/path").unwrap_err();
        assert!(matches!(err, DataError::BadUri { .. }));
    }

    #[test]
    fn reject_empty() {
        let err = parse("").unwrap_err();
        assert!(matches!(err, DataError::BadArgument { .. }));
    }

    #[test]
    fn parse_query_params() {
        let p = parse("https://host/path?a=1&b=two").unwrap();
        assert_eq!(p.query.get("a").map(String::as_str), Some("1"));
        assert_eq!(p.query.get("b").map(String::as_str), Some("two"));
    }
}
