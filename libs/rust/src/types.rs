use std::fs;
use std::path::Path;

use crate::error::{Result, SpadeError};

// ---------------------------------------------------------------------------
// Traits
// ---------------------------------------------------------------------------

/// Metadata about a Spade type used for manifest generation and output naming.
pub trait SpadeType {
    /// The Spade type string (e.g. `"file"`, `"directory"`, `"collection"`).
    fn type_name() -> &'static str;
    /// The default output subdirectory name (e.g. `"raster"`, `"files"`).
    fn default_output_name() -> &'static str;
    /// Manifest entry for the build function. Returns (type, format, item_type).
    fn manifest_entry() -> ManifestInfo;
}

/// Construct a typed value from raw filesystem input data.
pub trait FromInput: Sized {
    fn from_single_file(path: String) -> Result<Self>;
    fn from_multiple_files(paths: Vec<String>) -> Result<Self>;
    fn from_directory(path: String) -> Result<Self>;
}

/// Write a typed value to an output subdirectory.
pub trait IntoOutput {
    fn write_to(self: Box<Self>, output_dir: &Path) -> Result<()>;
    fn default_output_name(&self) -> &'static str;
}

/// Manifest metadata returned by `SpadeType::manifest_entry()`.
#[derive(Debug, Clone, PartialEq)]
pub struct ManifestInfo {
    pub type_name: &'static str,
    pub format: Option<&'static str>,
    pub item_type: Option<&'static str>,
}

// ---------------------------------------------------------------------------
// Macro to reduce boilerplate for single-file types
// ---------------------------------------------------------------------------

macro_rules! define_file_type {
    ($name:ident, $type_name:expr, $output_name:expr, $format:expr) => {
        #[derive(Clone, Debug, PartialEq)]
        pub struct $name {
            pub path: String,
        }

        impl $name {
            pub fn new(path: impl Into<String>) -> Self {
                Self { path: path.into() }
            }
        }

        impl SpadeType for $name {
            fn type_name() -> &'static str {
                $type_name
            }
            fn default_output_name() -> &'static str {
                $output_name
            }
            fn manifest_entry() -> ManifestInfo {
                ManifestInfo {
                    type_name: $type_name,
                    format: $format,
                    item_type: None,
                }
            }
        }

        impl FromInput for $name {
            fn from_single_file(path: String) -> Result<Self> {
                Ok(Self { path })
            }
            fn from_multiple_files(paths: Vec<String>) -> Result<Self> {
                Ok(Self {
                    path: paths
                        .into_iter()
                        .next()
                        .expect("from_multiple_files called with empty vec"),
                })
            }
            fn from_directory(_path: String) -> Result<Self> {
                Err(SpadeError::TypeMismatch {
                    name: String::new(),
                    expected: $type_name,
                    found: "directory",
                })
            }
        }

        impl IntoOutput for $name {
            fn write_to(self: Box<Self>, output_dir: &Path) -> Result<()> {
                fs::create_dir_all(output_dir)?;
                let src = Path::new(&self.path);
                let filename = src
                    .file_name()
                    .unwrap_or(src.as_os_str());
                fs::copy(src, output_dir.join(filename))?;
                Ok(())
            }
            fn default_output_name(&self) -> &'static str {
                $output_name
            }
        }
    };
}

macro_rules! define_collection_type {
    ($name:ident, $type_name:expr, $output_name:expr, $format:expr) => {
        #[derive(Clone, Debug, PartialEq)]
        pub struct $name {
            pub paths: Vec<String>,
        }

        impl $name {
            pub fn new(paths: Vec<String>) -> Self {
                Self { paths }
            }
        }

        impl SpadeType for $name {
            fn type_name() -> &'static str {
                "collection"
            }
            fn default_output_name() -> &'static str {
                $output_name
            }
            fn manifest_entry() -> ManifestInfo {
                ManifestInfo {
                    type_name: "collection",
                    format: $format,
                    item_type: Some($type_name),
                }
            }
        }

        impl FromInput for $name {
            fn from_single_file(path: String) -> Result<Self> {
                Ok(Self { paths: vec![path] })
            }
            fn from_multiple_files(paths: Vec<String>) -> Result<Self> {
                Ok(Self { paths })
            }
            fn from_directory(_path: String) -> Result<Self> {
                Err(SpadeError::TypeMismatch {
                    name: String::new(),
                    expected: "collection",
                    found: "directory",
                })
            }
        }

        impl IntoOutput for $name {
            fn write_to(self: Box<Self>, output_dir: &Path) -> Result<()> {
                fs::create_dir_all(output_dir)?;
                for file_path in &self.paths {
                    let src = Path::new(file_path);
                    let filename = src
                        .file_name()
                        .unwrap_or(src.as_os_str());
                    fs::copy(src, output_dir.join(filename))?;
                }
                Ok(())
            }
            fn default_output_name(&self) -> &'static str {
                $output_name
            }
        }
    };
}

// ---------------------------------------------------------------------------
// Single-file types
// ---------------------------------------------------------------------------

define_file_type!(File, "file", "file", None);
define_file_type!(RasterFile, "file", "raster", Some("GeoTIFF"));
define_file_type!(VectorFile, "file", "vector", Some("GeoJSON"));
define_file_type!(TabularFile, "file", "tabular", Some("CSV"));
define_file_type!(JsonFile, "json", "json", None);

// ---------------------------------------------------------------------------
// Directory type (special -- not a macro because of unique IntoOutput)
// ---------------------------------------------------------------------------

/// A directory-based input or output.
#[derive(Clone, Debug, PartialEq)]
pub struct Directory {
    pub path: String,
}

impl Directory {
    pub fn new(path: impl Into<String>) -> Self {
        Self { path: path.into() }
    }
}

impl SpadeType for Directory {
    fn type_name() -> &'static str {
        "directory"
    }
    fn default_output_name() -> &'static str {
        "directory"
    }
    fn manifest_entry() -> ManifestInfo {
        ManifestInfo {
            type_name: "directory",
            format: None,
            item_type: None,
        }
    }
}

impl FromInput for Directory {
    fn from_single_file(_path: String) -> Result<Self> {
        Err(SpadeError::TypeMismatch {
            name: String::new(),
            expected: "directory",
            found: "file",
        })
    }
    fn from_multiple_files(_paths: Vec<String>) -> Result<Self> {
        Err(SpadeError::TypeMismatch {
            name: String::new(),
            expected: "directory",
            found: "file",
        })
    }
    fn from_directory(path: String) -> Result<Self> {
        Ok(Self { path })
    }
}

impl IntoOutput for Directory {
    fn write_to(self: Box<Self>, output_dir: &Path) -> Result<()> {
        fs::create_dir_all(output_dir)?;
        copy_dir_recursive(Path::new(&self.path), output_dir)?;
        Ok(())
    }
    fn default_output_name(&self) -> &'static str {
        "directory"
    }
}

/// Recursively copy directory contents from `src` into `dst`.
pub(crate) fn copy_dir_recursive(src: &Path, dst: &Path) -> std::io::Result<()> {
    for entry in fs::read_dir(src)? {
        let entry = entry?;
        let file_type = entry.file_type()?;
        let dest_path = dst.join(entry.file_name());
        if file_type.is_file() {
            fs::copy(entry.path(), &dest_path)?;
        } else if file_type.is_dir() {
            fs::create_dir_all(&dest_path)?;
            copy_dir_recursive(&entry.path(), &dest_path)?;
        }
    }
    Ok(())
}

// ---------------------------------------------------------------------------
// Collection types
// ---------------------------------------------------------------------------

define_collection_type!(FileCollection, "file", "files", None);
define_collection_type!(RasterFileCollection, "file", "rasters", Some("GeoTIFF"));
define_collection_type!(VectorFileCollection, "file", "vectors", Some("GeoJSON"));
define_collection_type!(TabularFileCollection, "file", "tables", Some("CSV"));

// ---------------------------------------------------------------------------
// SpadeType impls for scalar types (used by ManifestBuilder)
// ---------------------------------------------------------------------------

impl SpadeType for String {
    fn type_name() -> &'static str {
        "string"
    }
    fn default_output_name() -> &'static str {
        "string"
    }
    fn manifest_entry() -> ManifestInfo {
        ManifestInfo {
            type_name: "string",
            format: None,
            item_type: None,
        }
    }
}

impl SpadeType for f64 {
    fn type_name() -> &'static str {
        "number"
    }
    fn default_output_name() -> &'static str {
        "number"
    }
    fn manifest_entry() -> ManifestInfo {
        ManifestInfo {
            type_name: "number",
            format: None,
            item_type: None,
        }
    }
}

impl SpadeType for f32 {
    fn type_name() -> &'static str {
        "number"
    }
    fn default_output_name() -> &'static str {
        "number"
    }
    fn manifest_entry() -> ManifestInfo {
        ManifestInfo {
            type_name: "number",
            format: None,
            item_type: None,
        }
    }
}

impl SpadeType for i64 {
    fn type_name() -> &'static str {
        "number"
    }
    fn default_output_name() -> &'static str {
        "number"
    }
    fn manifest_entry() -> ManifestInfo {
        ManifestInfo {
            type_name: "number",
            format: None,
            item_type: None,
        }
    }
}

impl SpadeType for i32 {
    fn type_name() -> &'static str {
        "number"
    }
    fn default_output_name() -> &'static str {
        "number"
    }
    fn manifest_entry() -> ManifestInfo {
        ManifestInfo {
            type_name: "number",
            format: None,
            item_type: None,
        }
    }
}

impl SpadeType for bool {
    fn type_name() -> &'static str {
        "boolean"
    }
    fn default_output_name() -> &'static str {
        "boolean"
    }
    fn manifest_entry() -> ManifestInfo {
        ManifestInfo {
            type_name: "boolean",
            format: None,
            item_type: None,
        }
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_file_new() {
        let f = File::new("/tmp/data.tif");
        assert_eq!(f.path, "/tmp/data.tif");
    }

    #[test]
    fn test_directory_new() {
        let d = Directory::new("/tmp/source");
        assert_eq!(d.path, "/tmp/source");
    }

    #[test]
    fn test_file_collection_new() {
        let fc = FileCollection::new(vec!["/tmp/a.tif".into(), "/tmp/b.tif".into()]);
        assert_eq!(fc.paths, vec!["/tmp/a.tif", "/tmp/b.tif"]);
    }

    #[test]
    fn test_raster_file_new() {
        let r = RasterFile::new("/tmp/raster.tif");
        assert_eq!(r.path, "/tmp/raster.tif");
    }

    #[test]
    fn test_vector_file_new() {
        let v = VectorFile::new("/tmp/vector.geojson");
        assert_eq!(v.path, "/tmp/vector.geojson");
    }

    #[test]
    fn test_tabular_file_new() {
        let t = TabularFile::new("/tmp/data.csv");
        assert_eq!(t.path, "/tmp/data.csv");
    }

    #[test]
    fn test_json_file_new() {
        let j = JsonFile::new("/tmp/data.json");
        assert_eq!(j.path, "/tmp/data.json");
    }

    #[test]
    fn test_raster_file_collection_new() {
        let rc = RasterFileCollection::new(vec!["/tmp/a.tif".into(), "/tmp/b.tif".into()]);
        assert_eq!(rc.paths, vec!["/tmp/a.tif", "/tmp/b.tif"]);
    }

    #[test]
    fn test_vector_file_collection_new() {
        let vc = VectorFileCollection::new(vec!["/tmp/a.geojson".into()]);
        assert_eq!(vc.paths, vec!["/tmp/a.geojson"]);
    }

    #[test]
    fn test_tabular_file_collection_new() {
        let tc = TabularFileCollection::new(vec!["/tmp/a.csv".into(), "/tmp/b.csv".into()]);
        assert_eq!(tc.paths, vec!["/tmp/a.csv", "/tmp/b.csv"]);
    }

    #[test]
    fn test_empty_collection() {
        let fc = FileCollection::new(vec![]);
        assert_eq!(fc.paths, Vec::<String>::new());
    }

    #[test]
    fn test_clone_produces_equal_values() {
        let f = File::new("/tmp/data.tif");
        let f2 = f.clone();
        assert_eq!(f, f2);

        let fc = FileCollection::new(vec!["/tmp/a.tif".into()]);
        let fc2 = fc.clone();
        assert_eq!(fc, fc2);

        let d = Directory::new("/tmp/source");
        let d2 = d.clone();
        assert_eq!(d, d2);
    }

    #[test]
    fn test_type_names() {
        assert_eq!(File::type_name(), "file");
        assert_eq!(RasterFile::type_name(), "file");
        assert_eq!(VectorFile::type_name(), "file");
        assert_eq!(TabularFile::type_name(), "file");
        assert_eq!(JsonFile::type_name(), "json");
        assert_eq!(Directory::type_name(), "directory");
        assert_eq!(FileCollection::type_name(), "collection");
        assert_eq!(RasterFileCollection::type_name(), "collection");
        assert_eq!(VectorFileCollection::type_name(), "collection");
        assert_eq!(TabularFileCollection::type_name(), "collection");
    }

    #[test]
    fn test_default_output_names() {
        assert_eq!(<File as SpadeType>::default_output_name(), "file");
        assert_eq!(<RasterFile as SpadeType>::default_output_name(), "raster");
        assert_eq!(<VectorFile as SpadeType>::default_output_name(), "vector");
        assert_eq!(<TabularFile as SpadeType>::default_output_name(), "tabular");
        assert_eq!(<JsonFile as SpadeType>::default_output_name(), "json");
        assert_eq!(<Directory as SpadeType>::default_output_name(), "directory");
        assert_eq!(<FileCollection as SpadeType>::default_output_name(), "files");
        assert_eq!(<RasterFileCollection as SpadeType>::default_output_name(), "rasters");
        assert_eq!(<VectorFileCollection as SpadeType>::default_output_name(), "vectors");
        assert_eq!(<TabularFileCollection as SpadeType>::default_output_name(), "tables");
    }

    #[test]
    fn test_manifest_entries() {
        let info = RasterFile::manifest_entry();
        assert_eq!(info.type_name, "file");
        assert_eq!(info.format, Some("GeoTIFF"));
        assert_eq!(info.item_type, None);

        let info = RasterFileCollection::manifest_entry();
        assert_eq!(info.type_name, "collection");
        assert_eq!(info.format, Some("GeoTIFF"));
        assert_eq!(info.item_type, Some("file"));

        let info = String::manifest_entry();
        assert_eq!(info.type_name, "string");
        assert_eq!(info.format, None);

        let info = f64::manifest_entry();
        assert_eq!(info.type_name, "number");

        let info = bool::manifest_entry();
        assert_eq!(info.type_name, "boolean");
    }
}
