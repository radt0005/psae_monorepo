//! Shared helpers for the `base` block collection.
//!
//! Every block reuses a small set of utilities: error types, tabular I/O,
//! parameter parsing, and the expansion-manifest representation used by
//! map blocks. Keeping these in one module avoids drift across blocks.

use std::fs;
use std::path::Path;

use polars::prelude::*;
use serde::{Deserialize, Serialize};
use thiserror::Error;

// ---------------------------------------------------------------------------
// Error type
// ---------------------------------------------------------------------------

#[derive(Debug, Error)]
pub enum BaseError {
    #[error("unsupported file format for '{0}'")]
    UnsupportedFormat(String),
    #[error("invalid filter expression: {0}")]
    BadExpression(String),
    #[error("schema mismatch: {0}")]
    SchemaMismatch(String),
    #[error("invalid aggregation spec: {0}")]
    InvalidAggregation(String),
    #[error("empty collection: at least one item is required")]
    EmptyCollection,
    #[error("missing parameter: {0}")]
    MissingParam(String),
    #[error(transparent)]
    Polars(#[from] polars::error::PolarsError),
    #[error(transparent)]
    Io(#[from] std::io::Error),
    #[error(transparent)]
    Json(#[from] serde_json::Error),
    #[error(transparent)]
    Yaml(#[from] serde_yaml::Error),
    #[error(transparent)]
    Spade(#[from] spade::SpadeError),
}

pub type Result<T> = std::result::Result<T, BaseError>;

// ---------------------------------------------------------------------------
// Tabular I/O
// ---------------------------------------------------------------------------

pub mod table {
    use super::*;

    fn ext_of(path: &str) -> String {
        Path::new(path)
            .extension()
            .and_then(|s| s.to_str())
            .map(|s| s.to_ascii_lowercase())
            .unwrap_or_default()
    }

    /// Read a tabular file (CSV or Parquet) into an eager `DataFrame`.
    pub fn read_table(path: &str) -> Result<DataFrame> {
        Ok(read_table_lazy(path)?.collect()?)
    }

    /// Read a tabular file (CSV or Parquet) into a `LazyFrame`.
    pub fn read_table_lazy(path: &str) -> Result<LazyFrame> {
        match ext_of(path).as_str() {
            "parquet" | "pq" => {
                let lf = LazyFrame::scan_parquet(path, ScanArgsParquet::default())?;
                Ok(lf)
            }
            "csv" | "tsv" | "txt" => {
                let separator = if ext_of(path) == "tsv" { b'\t' } else { b',' };
                let lf = LazyCsvReader::new(path)
                    .with_has_header(true)
                    .with_separator(separator)
                    .with_infer_schema_length(Some(1024))
                    .finish()?;
                Ok(lf)
            }
            other => Err(BaseError::UnsupportedFormat(format!(
                "extension '{other}' (path: {path})"
            ))),
        }
    }

    /// Write a `DataFrame` to Parquet.
    pub fn write_parquet(df: &mut DataFrame, path: &str) -> Result<()> {
        let file = std::fs::File::create(path)?;
        ParquetWriter::new(file)
            .with_compression(ParquetCompression::Snappy)
            .finish(df)?;
        Ok(())
    }

    /// Write a `DataFrame` to CSV.
    pub fn write_csv(
        df: &mut DataFrame,
        path: &str,
        delimiter: u8,
        has_header: bool,
    ) -> Result<()> {
        let file = std::fs::File::create(path)?;
        CsvWriter::new(file)
            .include_header(has_header)
            .with_separator(delimiter)
            .finish(df)?;
        Ok(())
    }
}

// ---------------------------------------------------------------------------
// Parameter parsing
// ---------------------------------------------------------------------------

pub mod params {
    use super::*;
    use serde::de::DeserializeOwned;

    /// Split a comma-separated string into trimmed, non-empty tokens.
    pub fn parse_csv_list(s: &str) -> Vec<String> {
        s.split(',')
            .map(|t| t.trim().to_string())
            .filter(|t| !t.is_empty())
            .collect()
    }

    /// Parse a JSON-encoded list into a typed `Vec<T>`.
    pub fn parse_json_list<T: DeserializeOwned>(s: &str) -> Result<Vec<T>> {
        Ok(serde_json::from_str::<Vec<T>>(s)?)
    }
}

// ---------------------------------------------------------------------------
// Expansion manifest (map-block output)
// ---------------------------------------------------------------------------

pub mod expansion {
    use super::*;

    #[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
    pub struct ExpansionItem {
        pub path: String,
        pub key: String,
    }

    #[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
    pub struct ExpansionManifest {
        pub items: Vec<ExpansionItem>,
    }

    /// Write an expansion manifest to `<base>/outputs/manifest/expansion.yaml`.
    pub fn write_manifest(base: &Path, items: Vec<ExpansionItem>) -> Result<()> {
        let dir = base.join("outputs").join("manifest");
        fs::create_dir_all(&dir)?;
        let manifest = ExpansionManifest { items };
        let yaml = serde_yaml::to_string(&manifest)?;
        fs::write(dir.join("expansion.yaml"), yaml)?;
        Ok(())
    }

    /// Materialise a list of scalar JSON values as per-item JSON files.
    ///
    /// Writes `<base>/outputs/manifest/items/<idx>.json` for each value and
    /// returns the corresponding `ExpansionItem`s (paths relative to `base`).
    pub fn materialise_scalar_items(
        base: &Path,
        values: &[serde_json::Value],
    ) -> Result<Vec<ExpansionItem>> {
        let items_dir = base.join("outputs").join("manifest").join("items");
        fs::create_dir_all(&items_dir)?;

        let width = ((values.len().max(1) as f64).log10().floor() as usize) + 1;
        let width = width.max(2);

        let mut out = Vec::with_capacity(values.len());
        for (idx, value) in values.iter().enumerate() {
            let filename = format!("{:0width$}.json", idx, width = width);
            let item_path = items_dir.join(&filename);
            let body = serde_json::json!({ "value": value });
            fs::write(&item_path, serde_json::to_string(&body)?)?;

            let rel_path = format!("outputs/manifest/items/{filename}");
            let key = stringify_scalar(value);
            out.push(ExpansionItem {
                path: rel_path,
                key,
            });
        }
        Ok(out)
    }

    fn stringify_scalar(v: &serde_json::Value) -> String {
        match v {
            serde_json::Value::String(s) => s.clone(),
            serde_json::Value::Number(n) => n.to_string(),
            serde_json::Value::Bool(b) => b.to_string(),
            serde_json::Value::Null => "null".to_string(),
            other => other.to_string(),
        }
    }
}

// ---------------------------------------------------------------------------
// Small utilities
// ---------------------------------------------------------------------------

/// Convert a Polars `DataFrame`'s column names into owned `String`s so callers
/// (especially tests) can compare them against plain `&str` literals without
/// fighting with Polars' `PlSmallStr`.
pub fn column_names(df: &polars::prelude::DataFrame) -> Vec<String> {
    df.get_column_names()
        .iter()
        .map(|s| s.to_string())
        .collect()
}

// ---------------------------------------------------------------------------
// Handler glue
// ---------------------------------------------------------------------------

/// Bridge a `BaseError` into the `Box<dyn Error>` shape `spade::run` expects.
pub fn boxed<T, E>(
    r: std::result::Result<T, E>,
) -> std::result::Result<T, Box<dyn std::error::Error + Send + Sync>>
where
    E: Into<Box<dyn std::error::Error + Send + Sync>>,
{
    r.map_err(Into::into)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    fn write_csv(path: &Path, content: &str) {
        fs::write(path, content).unwrap();
    }

    // -- params --

    #[test]
    fn parse_csv_list_splits_and_trims() {
        assert_eq!(
            params::parse_csv_list(" a , b,c ,, d"),
            vec!["a".to_string(), "b".into(), "c".into(), "d".into()]
        );
        assert!(params::parse_csv_list("").is_empty());
    }

    #[test]
    fn parse_json_list_ok() {
        let v: Vec<i64> = params::parse_json_list("[1, 2, 3]").unwrap();
        assert_eq!(v, vec![1, 2, 3]);
    }

    #[test]
    fn parse_json_list_bad() {
        let r: Result<Vec<i64>> = params::parse_json_list("not json");
        assert!(matches!(r, Err(BaseError::Json(_))));
    }

    // -- table --

    #[test]
    fn read_csv_roundtrip_to_parquet() {
        let tmp = TempDir::new().unwrap();
        let csv_path = tmp.path().join("in.csv");
        write_csv(&csv_path, "a,b\n1,x\n2,y\n3,z\n");
        let mut df = table::read_table(csv_path.to_str().unwrap()).unwrap();
        assert_eq!(df.height(), 3);

        let parquet_path = tmp.path().join("out.parquet");
        table::write_parquet(&mut df, parquet_path.to_str().unwrap()).unwrap();
        let df2 = table::read_table(parquet_path.to_str().unwrap()).unwrap();
        assert_eq!(df.height(), df2.height());
        assert_eq!(df.get_column_names(), df2.get_column_names());
    }

    #[test]
    fn read_table_bad_extension() {
        let r = table::read_table("/tmp/nope.xlsx");
        assert!(matches!(r, Err(BaseError::UnsupportedFormat(_))));
    }

    // -- expansion --

    #[test]
    fn materialise_and_write_manifest() {
        let tmp = TempDir::new().unwrap();
        let values = vec![
            serde_json::json!("NY"),
            serde_json::json!("MI"),
            serde_json::json!(42),
        ];
        let items = expansion::materialise_scalar_items(tmp.path(), &values).unwrap();
        assert_eq!(items.len(), 3);
        assert_eq!(items[0].key, "NY");
        assert_eq!(items[2].key, "42");
        assert!(items[0].path.starts_with("outputs/manifest/items/00"));

        expansion::write_manifest(tmp.path(), items).unwrap();
        let yaml = fs::read_to_string(tmp.path().join("outputs/manifest/expansion.yaml")).unwrap();
        assert!(yaml.contains("NY"));
        assert!(yaml.contains("MI"));
    }
}
