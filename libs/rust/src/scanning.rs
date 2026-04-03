use std::collections::HashMap;
use std::fs;
use std::path::Path;

use crate::error::{Result, SpadeError};
use crate::types::FromInput;

/// Raw representation of a scanned input entry from the `inputs/` directory.
#[derive(Debug, Clone)]
pub enum InputEntry {
    /// A single file was found in the input subdirectory.
    SingleFile(String),
    /// Multiple files were found in the input subdirectory.
    MultipleFiles(Vec<String>),
}

/// Load scalar parameters from `params.yaml` at the given base path.
pub fn load_params_from(base: &Path) -> Result<HashMap<String, serde_yaml::Value>> {
    let params_path = base.join("params.yaml");
    if !params_path.exists() {
        return Ok(HashMap::new());
    }
    let contents = fs::read_to_string(params_path)?;
    let value: serde_yaml::Value = serde_yaml::from_str(&contents)?;
    match value {
        serde_yaml::Value::Mapping(map) => {
            let mut result = HashMap::new();
            for (k, v) in map {
                if let serde_yaml::Value::String(key) = k {
                    result.insert(key, v);
                }
            }
            Ok(result)
        }
        serde_yaml::Value::Null => Ok(HashMap::new()),
        _ => Ok(HashMap::new()),
    }
}

/// Load scalar parameters from `params.yaml` in the current working directory.
pub fn load_params() -> Result<HashMap<String, serde_yaml::Value>> {
    load_params_from(Path::new("."))
}

/// Scan the `inputs/` directory at the given base path.
pub fn scan_inputs_from(base: &Path) -> Result<HashMap<String, InputEntry>> {
    let inputs_dir = base.join("inputs");
    if !inputs_dir.exists() {
        return Ok(HashMap::new());
    }

    let mut entries: Vec<_> = fs::read_dir(&inputs_dir)?
        .filter_map(|e| e.ok())
        .filter(|e| e.file_type().is_ok_and(|ft| ft.is_dir()))
        .collect();
    entries.sort_by_key(|e| e.file_name());

    let mut result = HashMap::new();
    for entry in entries {
        let name = entry.file_name().to_string_lossy().to_string();
        let subdir = entry.path();

        let mut files: Vec<String> = fs::read_dir(&subdir)?
            .filter_map(|e| e.ok())
            .filter(|e| e.file_type().is_ok_and(|ft| ft.is_file()))
            .map(|e| e.path().to_string_lossy().to_string())
            .collect();
        files.sort();

        if files.is_empty() {
            return Err(SpadeError::EmptyInputDir { name });
        }

        let input_entry = if files.len() == 1 {
            InputEntry::SingleFile(files.into_iter().next().unwrap())
        } else {
            InputEntry::MultipleFiles(files)
        };

        result.insert(name, input_entry);
    }

    Ok(result)
}

/// Scan the `inputs/` directory in the current working directory.
pub fn scan_inputs() -> Result<HashMap<String, InputEntry>> {
    scan_inputs_from(Path::new("."))
}

/// Merged arguments from `params.yaml` and `inputs/`, ready for handler consumption.
pub struct Args {
    params: HashMap<String, serde_yaml::Value>,
    inputs: HashMap<String, InputEntry>,
}

impl Args {
    /// Retrieve a typed file/directory input by name.
    pub fn input<T: FromInput>(&self, name: &str) -> Result<T> {
        let entry = self
            .inputs
            .get(name)
            .ok_or_else(|| SpadeError::InputNotFound {
                name: name.to_string(),
            })?;

        match entry {
            InputEntry::SingleFile(path) => T::from_single_file(path.clone()),
            InputEntry::MultipleFiles(paths) => T::from_multiple_files(paths.clone()),
        }
    }

    /// Retrieve a typed scalar parameter by name.
    pub fn param<T: serde::de::DeserializeOwned>(&self, name: &str) -> Result<T> {
        let value = self
            .params
            .get(name)
            .ok_or_else(|| SpadeError::ParamNotFound {
                name: name.to_string(),
            })?;

        serde_yaml::from_value(value.clone()).map_err(SpadeError::YamlError)
    }

    /// Check whether an input with the given name exists.
    pub fn has_input(&self, name: &str) -> bool {
        self.inputs.contains_key(name)
    }

    /// Check whether a parameter with the given name exists.
    pub fn has_param(&self, name: &str) -> bool {
        self.params.contains_key(name)
    }
}

/// Build the `Args` struct from the given base path.
pub fn build_args_from(base: &Path) -> Result<Args> {
    let params = load_params_from(base)?;
    let inputs = scan_inputs_from(base)?;
    Ok(Args { params, inputs })
}

/// Build the `Args` struct from the current working directory.
pub fn build_args() -> Result<Args> {
    build_args_from(Path::new("."))
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::{Directory, File, RasterFile, RasterFileCollection};
    use tempfile::TempDir;

    fn setup_work_dir() -> TempDir {
        let dir = TempDir::new().unwrap();
        fs::create_dir_all(dir.path().join("inputs")).unwrap();
        fs::create_dir_all(dir.path().join("outputs")).unwrap();
        fs::create_dir_all(dir.path().join("logs")).unwrap();
        dir
    }

    fn write_params(dir: &TempDir, content: &str) {
        fs::write(dir.path().join("params.yaml"), content).unwrap();
    }

    fn create_input_file(dir: &TempDir, name: &str, filename: &str, content: &[u8]) {
        let input_dir = dir.path().join("inputs").join(name);
        fs::create_dir_all(&input_dir).unwrap();
        fs::write(input_dir.join(filename), content).unwrap();
    }

    // -- load_params tests --

    #[test]
    fn test_load_params_basic() {
        let dir = setup_work_dir();
        write_params(&dir, "buffer_distance: 30\nmethod: bilinear\n");
        let params = load_params_from(dir.path()).unwrap();
        assert_eq!(
            params["buffer_distance"],
            serde_yaml::Value::Number(30.into())
        );
        assert_eq!(
            params["method"],
            serde_yaml::Value::String("bilinear".into())
        );
    }

    #[test]
    fn test_load_params_empty_file() {
        let dir = setup_work_dir();
        write_params(&dir, "");
        let params = load_params_from(dir.path()).unwrap();
        assert!(params.is_empty());
    }

    #[test]
    fn test_load_params_missing_file() {
        let dir = setup_work_dir();
        let params = load_params_from(dir.path()).unwrap();
        assert!(params.is_empty());
    }

    // -- scan_inputs tests --

    #[test]
    fn test_scan_inputs_single_file() {
        let dir = setup_work_dir();
        create_input_file(&dir, "raster", "data.tif", b"test data");
        let inputs = scan_inputs_from(dir.path()).unwrap();
        assert!(matches!(inputs["raster"], InputEntry::SingleFile(_)));
        if let InputEntry::SingleFile(path) = &inputs["raster"] {
            assert!(path.ends_with("data.tif"));
        }
    }

    #[test]
    fn test_scan_inputs_multiple_files() {
        let dir = setup_work_dir();
        create_input_file(&dir, "tiles", "001.tif", b"data");
        create_input_file(&dir, "tiles", "002.tif", b"data");
        create_input_file(&dir, "tiles", "003.tif", b"data");
        let inputs = scan_inputs_from(dir.path()).unwrap();
        assert!(matches!(inputs["tiles"], InputEntry::MultipleFiles(_)));
        if let InputEntry::MultipleFiles(paths) = &inputs["tiles"] {
            assert_eq!(paths.len(), 3);
        }
    }

    #[test]
    fn test_scan_inputs_empty_dir_error() {
        let dir = setup_work_dir();
        fs::create_dir_all(dir.path().join("inputs/empty")).unwrap();
        let result = scan_inputs_from(dir.path());
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, SpadeError::EmptyInputDir { .. }));
    }

    #[test]
    fn test_scan_inputs_no_inputs_dir() {
        let dir = TempDir::new().unwrap();
        // No inputs/ directory
        let inputs = scan_inputs_from(dir.path()).unwrap();
        assert!(inputs.is_empty());
    }

    #[test]
    fn test_scan_inputs_multiple_subdirs() {
        let dir = setup_work_dir();
        create_input_file(&dir, "reference", "ref.tif", b"ref");
        create_input_file(&dir, "target", "tgt.tif", b"tgt");
        let inputs = scan_inputs_from(dir.path()).unwrap();
        assert!(inputs.contains_key("reference"));
        assert!(inputs.contains_key("target"));
    }

    // -- Args tests --

    #[test]
    fn test_args_input_raster_file() {
        let dir = setup_work_dir();
        create_input_file(&dir, "source", "data.tif", b"test");
        let args = build_args_from(dir.path()).unwrap();
        let raster: RasterFile = args.input("source").unwrap();
        assert!(raster.path.ends_with("data.tif"));
    }

    #[test]
    fn test_args_input_collection() {
        let dir = setup_work_dir();
        create_input_file(&dir, "tiles", "a.tif", b"a");
        create_input_file(&dir, "tiles", "b.tif", b"b");
        let args = build_args_from(dir.path()).unwrap();
        let coll: RasterFileCollection = args.input("tiles").unwrap();
        assert_eq!(coll.paths.len(), 2);
    }

    #[test]
    fn test_args_input_not_found() {
        let dir = setup_work_dir();
        let args = build_args_from(dir.path()).unwrap();
        let result: Result<File> = args.input("missing");
        assert!(matches!(result, Err(SpadeError::InputNotFound { .. })));
    }

    #[test]
    fn test_args_param_f64() {
        let dir = setup_work_dir();
        write_params(&dir, "resolution: 10.5\n");
        let args = build_args_from(dir.path()).unwrap();
        let val: f64 = args.param("resolution").unwrap();
        assert!((val - 10.5).abs() < f64::EPSILON);
    }

    #[test]
    fn test_args_param_string() {
        let dir = setup_work_dir();
        write_params(&dir, "method: bilinear\n");
        let args = build_args_from(dir.path()).unwrap();
        let val: String = args.param("method").unwrap();
        assert_eq!(val, "bilinear");
    }

    #[test]
    fn test_args_param_bool() {
        let dir = setup_work_dir();
        write_params(&dir, "normalize: true\n");
        let args = build_args_from(dir.path()).unwrap();
        let val: bool = args.param("normalize").unwrap();
        assert!(val);
    }

    #[test]
    fn test_args_param_not_found() {
        let dir = setup_work_dir();
        let args = build_args_from(dir.path()).unwrap();
        let result: std::result::Result<f64, _> = args.param("missing");
        assert!(matches!(result, Err(SpadeError::ParamNotFound { .. })));
    }

    #[test]
    fn test_args_has_input() {
        let dir = setup_work_dir();
        create_input_file(&dir, "source", "data.tif", b"test");
        let args = build_args_from(dir.path()).unwrap();
        assert!(args.has_input("source"));
        assert!(!args.has_input("missing"));
    }

    #[test]
    fn test_args_has_param() {
        let dir = setup_work_dir();
        write_params(&dir, "buffer: 30\n");
        let args = build_args_from(dir.path()).unwrap();
        assert!(args.has_param("buffer"));
        assert!(!args.has_param("missing"));
    }

    #[test]
    fn test_build_args_params_and_inputs() {
        let dir = setup_work_dir();
        write_params(&dir, "buffer: 30\n");
        create_input_file(&dir, "raster", "data.tif", b"test");
        let args = build_args_from(dir.path()).unwrap();
        assert!(args.has_param("buffer"));
        assert!(args.has_input("raster"));
    }

    #[test]
    fn test_args_input_directory() {
        // Directory type is constructed via FromInput directly.
        let dir = Directory::from_directory("/tmp/source".into()).unwrap();
        assert_eq!(dir.path, "/tmp/source");
    }
}
