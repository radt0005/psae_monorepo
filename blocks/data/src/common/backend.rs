//! OpenDAL backend construction for parsed URIs.
//!
//! Each scheme maps to an [`opendal::services`] builder. Credentials come
//! from the environment for now — a future key-management service will
//! plug in here without changing block code.
//!
//! TODO(secrets): once KMS lands, replace the `env`-driven access-key
//! loading with a call to the secret provider and accept an optional
//! secret handle on each block.

use opendal::{services, Operator};

use crate::common::error::{DataError, Result};
use crate::common::uri::{ParsedUri, Scheme};

/// Build an OpenDAL [`Operator`] for the given URI.
pub fn build_operator(uri: &ParsedUri) -> Result<Operator> {
    match uri.scheme {
        Scheme::Fs => {
            // For `file:///abs/path`, the root is `/` and the object key
            // is the full absolute path. OpenDAL's fs service treats the
            // key as relative to root, which works correctly with root=`/`.
            let builder = services::Fs::default().root("/");
            Ok(Operator::new(builder)?.finish())
        }
        Scheme::Memory => {
            let builder = services::Memory::default();
            Ok(Operator::new(builder)?.finish())
        }
        Scheme::S3 => {
            let bucket = uri.bucket.as_deref().ok_or_else(|| DataError::BadUri {
                uri: uri.original.clone(),
                reason: "s3:// requires a bucket".into(),
            })?;
            let mut builder = services::S3::default().bucket(bucket);
            if let Ok(region) = std::env::var("AWS_REGION") {
                builder = builder.region(&region);
            }
            if let Ok(ak) = std::env::var("AWS_ACCESS_KEY_ID") {
                builder = builder.access_key_id(&ak);
            }
            if let Ok(sk) = std::env::var("AWS_SECRET_ACCESS_KEY") {
                builder = builder.secret_access_key(&sk);
            }
            if let Ok(ep) = std::env::var("AWS_ENDPOINT_URL") {
                builder = builder.endpoint(&ep);
            }
            Ok(Operator::new(builder)?.finish())
        }
        Scheme::Gcs => {
            let bucket = uri.bucket.as_deref().ok_or_else(|| DataError::BadUri {
                uri: uri.original.clone(),
                reason: "gs:// requires a bucket".into(),
            })?;
            let builder = services::Gcs::default().bucket(bucket);
            Ok(Operator::new(builder)?.finish())
        }
        Scheme::AzBlob => {
            let container = uri.bucket.as_deref().ok_or_else(|| DataError::BadUri {
                uri: uri.original.clone(),
                reason: "azblob:// requires a container".into(),
            })?;
            let mut builder = services::Azblob::default().container(container);
            if let Ok(ep) = std::env::var("AZURE_STORAGE_ENDPOINT") {
                builder = builder.endpoint(&ep);
            }
            if let Ok(acct) = std::env::var("AZURE_STORAGE_ACCOUNT") {
                builder = builder.account_name(&acct);
            }
            if let Ok(key) = std::env::var("AZURE_STORAGE_KEY") {
                builder = builder.account_key(&key);
            }
            Ok(Operator::new(builder)?.finish())
        }
        Scheme::Http | Scheme::Https => {
            let host = uri.host.as_deref().ok_or_else(|| DataError::BadUri {
                uri: uri.original.clone(),
                reason: "http(s):// requires a host".into(),
            })?;
            let scheme = if uri.scheme == Scheme::Https {
                "https"
            } else {
                "http"
            };
            let port_suffix = match uri.original.as_str() {
                s if s.starts_with(&format!("{scheme}://{host}:")) => {
                    // Keep the port intact by letting url crate give us the
                    // authority including port. Re-parse to get the port.
                    match url::Url::parse(&uri.original) {
                        Ok(u) => u
                            .port()
                            .map(|p| format!(":{p}"))
                            .unwrap_or_default(),
                        Err(_) => String::new(),
                    }
                }
                _ => String::new(),
            };
            let endpoint = format!("{scheme}://{host}{port_suffix}");
            let builder = services::Http::default().endpoint(&endpoint);
            Ok(Operator::new(builder)?.finish())
        }
        Scheme::Sftp => {
            let host = uri.host.as_deref().ok_or_else(|| DataError::BadUri {
                uri: uri.original.clone(),
                reason: "sftp:// requires a host".into(),
            })?;
            let endpoint = format!("sftp://{host}");
            let mut builder = services::Sftp::default().endpoint(&endpoint).root("/");
            if let Ok(user) = std::env::var("SFTP_USER") {
                builder = builder.user(&user);
            }
            Ok(Operator::new(builder)?.finish())
        }
        Scheme::Ftp => {
            // NOTE: the `services-ftp` feature is disabled on opendal
            // 0.50.2 because of a tls-stack mismatch in that crate.
            // Surface a clear error until we upgrade opendal.
            Err(DataError::UnsupportedScheme(
                "ftp (disabled in this build; pending opendal upgrade)".into(),
            ))
        }
        Scheme::Webdav => {
            let host = uri.host.as_deref().ok_or_else(|| DataError::BadUri {
                uri: uri.original.clone(),
                reason: "webdav:// requires a host".into(),
            })?;
            let endpoint = format!("https://{host}");
            let mut builder = services::Webdav::default().endpoint(&endpoint);
            if let Ok(user) = std::env::var("WEBDAV_USER") {
                builder = builder.username(&user);
            }
            if let Ok(pass) = std::env::var("WEBDAV_PASSWORD") {
                builder = builder.password(&pass);
            }
            Ok(Operator::new(builder)?.finish())
        }
        Scheme::GoogleDrive => {
            let token = std::env::var("GDRIVE_ACCESS_TOKEN").map_err(|_| {
                DataError::Unauthorized("GDRIVE_ACCESS_TOKEN must be set".into())
            })?;
            let builder = services::Gdrive::default().access_token(&token);
            Ok(Operator::new(builder)?.finish())
        }
        Scheme::OneDrive => {
            let token = std::env::var("ONEDRIVE_ACCESS_TOKEN").map_err(|_| {
                DataError::Unauthorized("ONEDRIVE_ACCESS_TOKEN must be set".into())
            })?;
            let builder = services::Onedrive::default().access_token(&token);
            Ok(Operator::new(builder)?.finish())
        }
        Scheme::Dropbox => {
            let token = std::env::var("DROPBOX_ACCESS_TOKEN").map_err(|_| {
                DataError::Unauthorized("DROPBOX_ACCESS_TOKEN must be set".into())
            })?;
            let builder = services::Dropbox::default().access_token(&token);
            Ok(Operator::new(builder)?.finish())
        }
    }
}

/// Return the key/path portion that `Operator` methods expect for this URI.
///
/// For `file://` and `memory://` the full path is the key. For bucketed
/// schemes (S3, GCS, Azure), the bucket is already baked into the
/// operator, so the key is everything after it. For HTTP(S), the key is
/// the path component (the host is the endpoint).
pub fn object_key(uri: &ParsedUri) -> String {
    match uri.scheme {
        Scheme::Fs => {
            // OpenDAL fs with root=`/` expects a relative key. Strip the
            // leading slash.
            uri.path.trim_start_matches('/').to_string()
        }
        Scheme::Memory => uri.path.trim_start_matches('/').to_string(),
        Scheme::S3 | Scheme::Gcs | Scheme::AzBlob => uri.path.clone(),
        Scheme::Http | Scheme::Https => {
            // Path already includes leading slash, which OpenDAL's http
            // service wants stripped.
            uri.path.trim_start_matches('/').to_string()
        }
        Scheme::Sftp | Scheme::Ftp | Scheme::Webdav => uri.path.clone(),
        Scheme::GoogleDrive | Scheme::OneDrive | Scheme::Dropbox => uri.path.clone(),
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::common::uri::parse;

    #[test]
    fn build_memory() {
        let u = parse("memory:///abc").unwrap();
        let op = build_operator(&u).unwrap();
        assert_eq!(op.info().scheme(), opendal::Scheme::Memory);
    }

    #[test]
    fn build_fs() {
        let u = parse("file:///tmp/x").unwrap();
        let op = build_operator(&u).unwrap();
        assert_eq!(op.info().scheme(), opendal::Scheme::Fs);
    }

    #[test]
    fn key_for_fs_strips_leading_slash() {
        let u = parse("/tmp/x").unwrap();
        assert_eq!(object_key(&u), "tmp/x");
    }

    #[test]
    fn key_for_s3_is_path_component() {
        let u = parse("s3://bkt/path/to/key").unwrap();
        assert_eq!(object_key(&u), "path/to/key");
    }

    #[test]
    fn key_for_http_strips_leading_slash() {
        let u = parse("https://example.com/data.csv").unwrap();
        assert_eq!(object_key(&u), "data.csv");
    }
}
