use std::collections::HashMap;
use std::env;
use std::fs;
use std::path::Path;

use crate::error::Result;
use crate::types::{IntoOutput, SpadeType};

/// A collection of named outputs for handlers that produce multiple results.
///
/// # Example
///
/// ```ignore
/// use spade::{Outputs, RasterFile, JsonFile};
///
/// let mut outputs = Outputs::new();
/// outputs.add("raster", RasterFile::new("result.tif"));
/// outputs.add("stats", JsonFile::new("stats.json"));
/// ```
pub struct Outputs {
    entries: Vec<(String, Box<dyn IntoOutput>)>,
}

impl Outputs {
    /// Create an empty `Outputs` collection.
    pub fn new() -> Self {
        Self {
            entries: Vec::new(),
        }
    }

    /// Add a named output to the collection.
    pub fn add<T: IntoOutput + 'static>(&mut self, name: impl Into<String>, value: T) {
        self.entries.push((name.into(), Box::new(value)));
    }

    /// Create an `Outputs` with a single entry, using the type's default output name.
    pub fn single<T: IntoOutput + SpadeType + 'static>(value: T) -> Self {
        let name = <T as SpadeType>::default_output_name().to_string();
        let mut out = Self::new();
        out.entries.push((name, Box::new(value)));
        out
    }
}

impl Default for Outputs {
    fn default() -> Self {
        Self::new()
    }
}

impl IntoOutput for Outputs {
    fn write_to(self: Box<Self>, output_dir: &Path) -> Result<()> {
        fs::create_dir_all(output_dir)?;
        for (name, value) in self.entries {
            let dir = output_dir.join(&name);
            value.write_to(&dir)?;
        }
        Ok(())
    }

    fn default_output_name(&self) -> &'static str {
        "__outputs__"
    }
}

impl SpadeType for Outputs {
    fn type_name() -> &'static str {
        "__outputs__"
    }
    fn default_output_name() -> &'static str {
        "__outputs__"
    }
    fn manifest_entry() -> crate::types::ManifestInfo {
        crate::types::ManifestInfo {
            type_name: "__outputs__",
            format: None,
            item_type: None,
        }
    }
}

/// Read the block manifest to get output declarations.
///
/// Checks (in order):
/// 1. `SPADE_BLOCK_MANIFEST` environment variable
/// 2. `block.yaml` in the given base directory
///
/// Returns `None` if no manifest is found.
pub fn read_block_manifest_from(base: &Path) -> Option<HashMap<String, serde_yaml::Value>> {
    // Check env var first
    if let Ok(manifest_path) = env::var("SPADE_BLOCK_MANIFEST") {
        let path = Path::new(&manifest_path);
        if path.exists()
            && let Ok(contents) = fs::read_to_string(path)
            && let Ok(value) = serde_yaml::from_str::<serde_yaml::Value>(&contents)
            && let Some(outputs) = value.get("outputs")
        {
            return yaml_value_to_map(outputs);
        }
    }

    // Check block.yaml in base dir
    let block_yaml = base.join("block.yaml");
    if block_yaml.exists()
        && let Ok(contents) = fs::read_to_string(block_yaml)
        && let Ok(value) = serde_yaml::from_str::<serde_yaml::Value>(&contents)
        && let Some(outputs) = value.get("outputs")
    {
        return yaml_value_to_map(outputs);
    }

    None
}

/// Read the block manifest from the current working directory.
pub fn read_block_manifest() -> Option<HashMap<String, serde_yaml::Value>> {
    read_block_manifest_from(Path::new("."))
}

fn yaml_value_to_map(value: &serde_yaml::Value) -> Option<HashMap<String, serde_yaml::Value>> {
    if let serde_yaml::Value::Mapping(map) = value {
        let mut result = HashMap::new();
        for (k, v) in map {
            if let serde_yaml::Value::String(key) = k {
                result.insert(key.clone(), v.clone());
            }
        }
        Some(result)
    } else {
        None
    }
}

/// Write handler output(s) to `<base>/outputs/`.
pub fn write_outputs_to<T: IntoOutput>(
    result: T,
    base: &Path,
    manifest_outputs: Option<&HashMap<String, serde_yaml::Value>>,
) -> Result<()> {
    let boxed: Box<dyn IntoOutput> = Box::new(result);
    let output_name = boxed.default_output_name();

    let outputs_root = base.join("outputs");

    // If this is an Outputs collection, it handles its own subdirectory structure
    if output_name == "__outputs__" {
        boxed.write_to(&outputs_root)?;
        return Ok(());
    }

    // Unit/no-return handler
    if output_name == "__none__" {
        return Ok(());
    }

    // Single output: determine the output directory name
    let name = if let Some(manifest) = manifest_outputs {
        if manifest.len() == 1 {
            manifest.keys().next().unwrap().clone()
        } else {
            output_name.to_string()
        }
    } else {
        output_name.to_string()
    };

    let output_dir = outputs_root.join(&name);
    boxed.write_to(&output_dir)?;
    Ok(())
}

/// Write handler output(s) to `outputs/` in the current working directory.
pub fn write_outputs<T: IntoOutput>(
    result: T,
    manifest_outputs: Option<&HashMap<String, serde_yaml::Value>>,
) -> Result<()> {
    write_outputs_to(result, Path::new("."), manifest_outputs)
}

/// Unit type output for handlers that return nothing.
impl IntoOutput for () {
    fn write_to(self: Box<Self>, _output_dir: &Path) -> Result<()> {
        Ok(())
    }
    fn default_output_name(&self) -> &'static str {
        "__none__"
    }
}

impl SpadeType for () {
    fn type_name() -> &'static str {
        "__none__"
    }
    fn default_output_name() -> &'static str {
        "__none__"
    }
    fn manifest_entry() -> crate::types::ManifestInfo {
        crate::types::ManifestInfo {
            type_name: "__none__",
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
    use crate::types::{Directory, File, JsonFile, RasterFile, RasterFileCollection, VectorFile};
    use tempfile::TempDir;

    fn setup_work_dir() -> TempDir {
        let dir = TempDir::new().unwrap();
        fs::create_dir_all(dir.path().join("inputs")).unwrap();
        fs::create_dir_all(dir.path().join("outputs")).unwrap();
        fs::create_dir_all(dir.path().join("logs")).unwrap();
        dir
    }

    #[test]
    fn test_write_single_raster_file() {
        let dir = setup_work_dir();
        let src = dir.path().join("tmp_result.tif");
        fs::write(&src, b"raster data").unwrap();

        let result = RasterFile::new(src.to_string_lossy());
        write_outputs_to(result, dir.path(), None).unwrap();

        let output_dir = dir.path().join("outputs/raster");
        assert!(output_dir.exists());
        assert!(output_dir.join("tmp_result.tif").exists());
        assert_eq!(
            fs::read(output_dir.join("tmp_result.tif")).unwrap(),
            b"raster data"
        );
    }

    #[test]
    fn test_write_single_file() {
        let dir = setup_work_dir();
        let src = dir.path().join("result.dat");
        fs::write(&src, b"file data").unwrap();

        let result = File::new(src.to_string_lossy());
        write_outputs_to(result, dir.path(), None).unwrap();

        assert!(dir.path().join("outputs/file/result.dat").exists());
    }

    #[test]
    fn test_write_file_collection() {
        let dir = setup_work_dir();
        let mut paths = Vec::new();
        for i in 0..3 {
            let p = dir.path().join(format!("tile_{i}.tif"));
            fs::write(&p, format!("tile {i}")).unwrap();
            paths.push(p.to_string_lossy().to_string());
        }

        let result = RasterFileCollection::new(paths);
        write_outputs_to(result, dir.path(), None).unwrap();

        let output_dir = dir.path().join("outputs/rasters");
        assert!(output_dir.exists());
        assert_eq!(
            fs::read_dir(&output_dir)
                .unwrap()
                .filter_map(|e| e.ok())
                .count(),
            3
        );
    }

    #[test]
    fn test_write_directory_output() {
        let dir = setup_work_dir();
        let src_dir = dir.path().join("result_dir");
        fs::create_dir_all(&src_dir).unwrap();
        fs::write(src_dir.join("file1.txt"), b"a").unwrap();
        fs::write(src_dir.join("file2.txt"), b"b").unwrap();

        let result = Directory::new(src_dir.to_string_lossy());
        write_outputs_to(result, dir.path(), None).unwrap();

        let output_dir = dir.path().join("outputs/directory");
        assert!(output_dir.exists());
        assert!(output_dir.join("file1.txt").exists());
        assert!(output_dir.join("file2.txt").exists());
    }

    #[test]
    fn test_write_with_manifest_custom_name() {
        let dir = setup_work_dir();
        let src = dir.path().join("result.tif");
        fs::write(&src, b"data").unwrap();

        let mut manifest = HashMap::new();
        manifest.insert(
            "custom_output".to_string(),
            serde_yaml::to_value(&serde_yaml::Mapping::from_iter([(
                serde_yaml::Value::String("type".into()),
                serde_yaml::Value::String("file".into()),
            )]))
            .unwrap(),
        );

        let result = RasterFile::new(src.to_string_lossy());
        write_outputs_to(result, dir.path(), Some(&manifest)).unwrap();

        assert!(dir
            .path()
            .join("outputs/custom_output/result.tif")
            .exists());
    }

    #[test]
    fn test_write_multiple_named_outputs() {
        let dir = setup_work_dir();
        let raster_src = dir.path().join("result.tif");
        fs::write(&raster_src, b"raster").unwrap();
        let json_src = dir.path().join("summary.json");
        fs::write(&json_src, b"{\"key\": \"value\"}").unwrap();

        let mut outputs = Outputs::new();
        outputs.add("raster", RasterFile::new(raster_src.to_string_lossy()));
        outputs.add("summary", JsonFile::new(json_src.to_string_lossy()));

        write_outputs_to(outputs, dir.path(), None).unwrap();

        assert!(dir.path().join("outputs/raster/result.tif").exists());
        assert!(dir.path().join("outputs/summary/summary.json").exists());
    }

    #[test]
    fn test_preserves_filename() {
        let dir = setup_work_dir();
        let src = dir.path().join("my_custom_name.geojson");
        fs::write(&src, b"geojson data").unwrap();

        let result = VectorFile::new(src.to_string_lossy());
        write_outputs_to(result, dir.path(), None).unwrap();

        assert!(dir
            .path()
            .join("outputs/vector/my_custom_name.geojson")
            .exists());
    }

    #[test]
    fn test_write_unit_output() {
        let dir = setup_work_dir();
        write_outputs_to((), dir.path(), None).unwrap();
        let output_files: Vec<_> = fs::read_dir(dir.path().join("outputs"))
            .unwrap()
            .filter_map(|e| e.ok())
            .collect();
        assert!(output_files.is_empty());
    }

    // -- read_block_manifest tests --
    // Tests that manipulate SPADE_BLOCK_MANIFEST env var must be serialized
    // since env vars are process-global.
    static ENV_MUTEX: std::sync::Mutex<()> = std::sync::Mutex::new(());

    #[test]
    fn test_read_manifest_none() {
        let _lock = ENV_MUTEX.lock().unwrap();
        // Ensure env var is clear
        unsafe { env::remove_var("SPADE_BLOCK_MANIFEST") };
        let dir = setup_work_dir();
        assert!(read_block_manifest_from(dir.path()).is_none());
    }

    #[test]
    fn test_read_manifest_block_yaml() {
        let _lock = ENV_MUTEX.lock().unwrap();
        unsafe { env::remove_var("SPADE_BLOCK_MANIFEST") };
        let dir = setup_work_dir();
        let manifest_content =
            "id: test.block\noutputs:\n  raster:\n    type: file\n    format: GeoTIFF\n";
        fs::write(dir.path().join("block.yaml"), manifest_content).unwrap();

        let result = read_block_manifest_from(dir.path()).unwrap();
        assert!(result.contains_key("raster"));
    }

    #[test]
    fn test_read_manifest_env_var() {
        let _lock = ENV_MUTEX.lock().unwrap();
        let dir = setup_work_dir();
        let manifest_path = dir.path().join("external_block.yaml");
        let manifest_content = "outputs:\n  output:\n    type: file\n";
        fs::write(&manifest_path, manifest_content).unwrap();

        // SAFETY: test-only env var manipulation, serialized by mutex
        unsafe {
            env::set_var(
                "SPADE_BLOCK_MANIFEST",
                manifest_path.to_string_lossy().as_ref(),
            )
        };
        let result = read_block_manifest_from(dir.path()).unwrap();
        unsafe { env::remove_var("SPADE_BLOCK_MANIFEST") };

        assert!(result.contains_key("output"));
    }

    #[test]
    fn test_read_manifest_env_var_takes_precedence() {
        let _lock = ENV_MUTEX.lock().unwrap();
        let dir = setup_work_dir();

        // block.yaml in work dir
        let cwd_content = "outputs:\n  cwd_output:\n    type: file\n";
        fs::write(dir.path().join("block.yaml"), cwd_content).unwrap();

        // external manifest
        let ext_path = dir.path().join("external.yaml");
        let ext_content = "outputs:\n  env_output:\n    type: file\n";
        fs::write(&ext_path, ext_content).unwrap();

        // SAFETY: test-only env var manipulation, serialized by mutex
        unsafe {
            env::set_var(
                "SPADE_BLOCK_MANIFEST",
                ext_path.to_string_lossy().as_ref(),
            )
        };
        let result = read_block_manifest_from(dir.path()).unwrap();
        unsafe { env::remove_var("SPADE_BLOCK_MANIFEST") };

        assert!(result.contains_key("env_output"));
        assert!(!result.contains_key("cwd_output"));
    }
}
