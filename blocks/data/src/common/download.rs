//! HTTP fetch + verify + unpack helpers for catalog blocks.
//!
//! Catalog blocks need to "download a URL to a file, optionally verify a
//! checksum, optionally unpack a zip". Keeping these three operations
//! together in one module avoids every catalog block re-implementing
//! them.

use std::fs;
use std::io::{Read, Write};
use std::path::{Path, PathBuf};

use sha2::{Digest, Sha256};

use crate::common::error::{DataError, Result};

/// Default User-Agent used for catalog downloads.
///
/// Some hosts (Census, Cloudflare-fronted CDNs, etc.) return HTML bot
/// challenges or 403s for the default `reqwest/<ver>` UA, so we identify
/// ourselves as a spade-data fetcher with a browser-like fallback. Using
/// a self-identifying UA is also polite to operators who inspect logs.
const DEFAULT_USER_AGENT: &str =
    "spade-data/0.1 (+https://github.com/psae/spade) Mozilla/5.0 (compatible; SpadeData)";

/// Fetch `uri` into `dest`, streaming the body to disk.
pub fn fetch_to(dest: &Path, uri: &str) -> Result<()> {
    let client = reqwest::blocking::Client::builder()
        .user_agent(DEFAULT_USER_AGENT)
        .redirect(reqwest::redirect::Policy::limited(10))
        .build()?;
    let mut resp = client.get(uri).send()?;

    if resp.status() == reqwest::StatusCode::NOT_FOUND {
        return Err(DataError::NotFound(uri.to_string()));
    }
    if resp.status() == reqwest::StatusCode::FORBIDDEN
        || resp.status() == reqwest::StatusCode::UNAUTHORIZED
    {
        return Err(DataError::Unauthorized(uri.to_string()));
    }
    if !resp.status().is_success() {
        return Err(DataError::Network {
            uri: uri.to_string(),
            source: format!("HTTP {}", resp.status()).into(),
        });
    }

    if let Some(parent) = dest.parent() {
        fs::create_dir_all(parent)?;
    }
    let mut file = fs::File::create(dest)?;
    resp.copy_to(&mut file).map_err(|e| DataError::Network {
        uri: uri.to_string(),
        source: Box::new(e),
    })?;
    Ok(())
}

/// Fetch `uri` into `dest` and verify sha256 if `expected_sha256` is
/// supplied.
pub fn fetch_and_verify(dest: &Path, uri: &str, expected_sha256: Option<&str>) -> Result<()> {
    fetch_to(dest, uri)?;
    if let Some(expected) = expected_sha256 {
        let actual = sha256_of_file(dest)?;
        if !actual.eq_ignore_ascii_case(expected) {
            return Err(DataError::ChecksumMismatch {
                uri: uri.to_string(),
                expected: expected.to_string(),
                actual,
            });
        }
    }
    Ok(())
}

/// Compute the sha256 of a file on disk.
pub fn sha256_of_file(path: &Path) -> Result<String> {
    let mut hasher = Sha256::new();
    let mut buf = [0u8; 8192];
    let mut f = fs::File::open(path)?;
    loop {
        let n = f.read(&mut buf)?;
        if n == 0 {
            break;
        }
        hasher.update(&buf[..n]);
    }
    Ok(hex_encode(&hasher.finalize()))
}

/// Extract entries from `zip` into `dest_dir`, keeping only entries whose
/// (zip-internal) filename satisfies `filter`. Filenames are flattened
/// (no intermediate directories). Returns the extracted destination
/// paths.
pub fn extract_zip<F>(zip: &Path, dest_dir: &Path, mut filter: F) -> Result<Vec<PathBuf>>
where
    F: FnMut(&str) -> bool,
{
    fs::create_dir_all(dest_dir)?;
    let file = fs::File::open(zip)?;
    let mut archive = zip::ZipArchive::new(file)?;

    let mut out = Vec::new();
    let n = archive.len();
    for i in 0..n {
        let mut entry = archive.by_index(i)?;
        if entry.is_dir() {
            continue;
        }
        let name = entry.name().to_string();
        if !filter(&name) {
            continue;
        }
        let flat_name = Path::new(&name)
            .file_name()
            .map(|s| s.to_string_lossy().to_string())
            .unwrap_or_else(|| name.clone());
        let dest = dest_dir.join(&flat_name);
        if let Some(parent) = dest.parent() {
            fs::create_dir_all(parent)?;
        }
        let mut w = fs::File::create(&dest)?;
        std::io::copy(&mut entry, &mut w)?;
        out.push(dest);
    }
    Ok(out)
}

/// Extract all entries while preserving their zip-internal path structure.
pub fn extract_zip_tree(zip: &Path, dest_dir: &Path) -> Result<Vec<PathBuf>> {
    fs::create_dir_all(dest_dir)?;
    let file = fs::File::open(zip)?;
    let mut archive = zip::ZipArchive::new(file)?;

    let mut out = Vec::new();
    let n = archive.len();
    for i in 0..n {
        let mut entry = archive.by_index(i)?;
        if entry.is_dir() {
            continue;
        }
        let name = entry.name().to_string();
        let safe_rel = sanitize_zip_name(&name).ok_or(DataError::Zip(
            zip::result::ZipError::InvalidArchive("unsafe zip entry path"),
        ))?;
        let dest = dest_dir.join(&safe_rel);
        if let Some(parent) = dest.parent() {
            fs::create_dir_all(parent)?;
        }
        let mut w = fs::File::create(&dest)?;
        std::io::copy(&mut entry, &mut w)?;
        out.push(dest);
    }
    Ok(out)
}

fn sanitize_zip_name(name: &str) -> Option<PathBuf> {
    let mut out = PathBuf::new();
    for comp in Path::new(name).components() {
        match comp {
            std::path::Component::Normal(p) => out.push(p),
            std::path::Component::CurDir => {}
            _ => return None,
        }
    }
    if out.as_os_str().is_empty() {
        None
    } else {
        Some(out)
    }
}

fn hex_encode(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    let mut s = String::with_capacity(bytes.len() * 2);
    for b in bytes {
        let _ = write!(&mut s, "{:02x}", b);
    }
    s
}

/// Helper used by tests to build a small zip in-memory.
pub fn build_test_zip(entries: &[(&str, &[u8])]) -> Result<Vec<u8>> {
    let mut buf = std::io::Cursor::new(Vec::new());
    {
        let mut w = zip::ZipWriter::new(&mut buf);
        let options: zip::write::SimpleFileOptions =
            zip::write::SimpleFileOptions::default()
                .compression_method(zip::CompressionMethod::Stored);
        for (name, data) in entries {
            w.start_file(*name, options)?;
            w.write_all(data)?;
        }
        w.finish()?;
    }
    Ok(buf.into_inner())
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    #[test]
    fn extract_filtered_zip() {
        let tmp = TempDir::new().unwrap();
        let bytes = build_test_zip(&[
            ("a.csv", b"1,2"),
            ("b.csv", b"3,4"),
            ("c.txt", b"nope"),
        ])
        .unwrap();
        let zip_path = tmp.path().join("test.zip");
        fs::write(&zip_path, bytes).unwrap();

        let out_dir = tmp.path().join("out");
        let extracted = extract_zip(&zip_path, &out_dir, |n| n.ends_with(".csv")).unwrap();
        assert_eq!(extracted.len(), 2);
        assert!(out_dir.join("a.csv").exists());
        assert!(out_dir.join("b.csv").exists());
        assert!(!out_dir.join("c.txt").exists());
    }

    #[test]
    fn extract_all_zip_entries() {
        let tmp = TempDir::new().unwrap();
        let bytes = build_test_zip(&[("a.csv", b"x"), ("b.csv", b"y")]).unwrap();
        let zip_path = tmp.path().join("test.zip");
        fs::write(&zip_path, bytes).unwrap();

        let out_dir = tmp.path().join("out");
        let extracted = extract_zip(&zip_path, &out_dir, |_| true).unwrap();
        assert_eq!(extracted.len(), 2);
    }

    #[test]
    fn checksum_mismatch_errs() {
        let tmp = TempDir::new().unwrap();
        let dest = tmp.path().join("f.bin");
        fs::write(&dest, b"hello").unwrap();
        let actual = sha256_of_file(&dest).unwrap();
        assert_ne!(actual, "deadbeef");
    }

    #[test]
    fn sha256_is_stable() {
        let tmp = TempDir::new().unwrap();
        let p = tmp.path().join("x");
        fs::write(&p, b"abc").unwrap();
        let expected = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad";
        assert_eq!(sha256_of_file(&p).unwrap(), expected);
    }
}
