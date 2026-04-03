use std::path::Path;

use crate::output::{read_block_manifest_from, write_outputs_to};
use crate::scanning::{build_args_from, Args};
use crate::types::{IntoOutput, SpadeType};

/// Execute a handler function as a Spade block.
///
/// This is the main entry point for block execution. It:
/// 1. Loads scalar parameters from `params.yaml`
/// 2. Scans the `inputs/` directory for file-based arguments
/// 3. Builds the `Args` struct
/// 4. Calls the handler function
/// 5. Writes return value(s) to the `outputs/` directory
///
/// If the handler or any step fails, the process exits with code 1.
///
/// # Example
///
/// ```ignore
/// use spade::{run, Args, RasterFile};
///
/// fn handler(args: Args) -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
///     let source: RasterFile = args.input("source")?;
///     Ok(RasterFile::new("result.tif"))
/// }
///
/// fn main() {
///     run(handler);
/// }
/// ```
pub fn run<F, O>(handler: F)
where
    F: FnOnce(Args) -> std::result::Result<O, Box<dyn std::error::Error + Send + Sync>>,
    O: IntoOutput + SpadeType + 'static,
{
    if let Err(e) = run_at(Path::new("."), handler) {
        eprintln!("spade: {e}");
        std::process::exit(1);
    }
}

/// Run a handler at a specific base path. Used for testing.
fn run_at<F, O>(
    base: &Path,
    handler: F,
) -> std::result::Result<(), Box<dyn std::error::Error + Send + Sync>>
where
    F: FnOnce(Args) -> std::result::Result<O, Box<dyn std::error::Error + Send + Sync>>,
    O: IntoOutput + SpadeType + 'static,
{
    let args = build_args_from(base)?;
    let result = handler(args)?;

    if O::type_name() == "__none__" {
        return Ok(());
    }

    let manifest = read_block_manifest_from(base);
    write_outputs_to(result, base, manifest.as_ref())?;
    Ok(())
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::output::Outputs;
    use crate::types::{File, JsonFile, RasterFile};
    use std::fs;
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

    #[test]
    fn test_simple_handler_no_output() {
        let dir = setup_work_dir();
        create_input_file(&dir, "source", "data.tif", b"test data");

        let called = std::sync::Arc::new(std::sync::Mutex::new(false));
        let called_clone = called.clone();

        run_at(
            dir.path(),
            move |args: Args| -> std::result::Result<(), Box<dyn std::error::Error + Send + Sync>> {
                let _source: File = args.input("source")?;
                *called_clone.lock().unwrap() = true;
                Ok(())
            },
        )
        .unwrap();

        assert!(*called.lock().unwrap());
        let output_files: Vec<_> = fs::read_dir(dir.path().join("outputs"))
            .unwrap()
            .filter_map(|e| e.ok())
            .collect();
        assert!(output_files.is_empty());
    }

    #[test]
    fn test_handler_with_params_and_inputs() {
        let dir = setup_work_dir();
        write_params(&dir, "buffer: 30\nmethod: bilinear\n");
        create_input_file(&dir, "raster", "data.tif", b"test data");

        run_at(
            dir.path(),
            move |args: Args| -> std::result::Result<(), Box<dyn std::error::Error + Send + Sync>> {
                let _raster: RasterFile = args.input("raster")?;
                let buffer: i64 = args.param("buffer")?;
                let method: String = args.param("method")?;
                assert_eq!(buffer, 30);
                assert_eq!(method, "bilinear");
                Ok(())
            },
        )
        .unwrap();
    }

    #[test]
    fn test_handler_returning_raster_file() {
        let dir = setup_work_dir();
        create_input_file(&dir, "source", "data.tif", b"test data");

        let result_path = dir.path().join("processed.tif");
        fs::write(&result_path, b"processed data").unwrap();
        let rp = result_path.to_string_lossy().to_string();

        run_at(
            dir.path(),
            move |args: Args| -> std::result::Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
                let _source: RasterFile = args.input("source")?;
                Ok(RasterFile::new(rp))
            },
        )
        .unwrap();

        let output_files: Vec<_> = walkdir(dir.path().join("outputs"));
        assert_eq!(output_files.len(), 1);
        assert_eq!(fs::read(&output_files[0]).unwrap(), b"processed data");
    }

    #[test]
    fn test_handler_returning_outputs() {
        let dir = setup_work_dir();
        create_input_file(&dir, "source", "data.tif", b"test data");

        let raster_path = dir.path().join("result.tif");
        fs::write(&raster_path, b"raster").unwrap();
        let json_path = dir.path().join("stats.json");
        fs::write(&json_path, b"{\"mean\": 42}").unwrap();
        let rp = raster_path.to_string_lossy().to_string();
        let jp = json_path.to_string_lossy().to_string();

        run_at(
            dir.path(),
            move |_args: Args| -> std::result::Result<Outputs, Box<dyn std::error::Error + Send + Sync>> {
                let mut outputs = Outputs::new();
                outputs.add("raster", RasterFile::new(&rp));
                outputs.add("stats", JsonFile::new(&jp));
                Ok(outputs)
            },
        )
        .unwrap();

        assert!(dir.path().join("outputs/raster/result.tif").exists());
        assert!(dir.path().join("outputs/stats/stats.json").exists());
    }

    #[test]
    fn test_handler_error_propagates() {
        let dir = setup_work_dir();
        create_input_file(&dir, "source", "data.tif", b"test data");

        let result = run_at(
            dir.path(),
            move |_args: Args| -> std::result::Result<(), Box<dyn std::error::Error + Send + Sync>> {
                Err("processing failed".into())
            },
        );

        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .to_string()
            .contains("processing failed"));
    }

    #[test]
    fn test_full_workflow() {
        let dir = setup_work_dir();
        write_params(&dir, "resolution: 10\nmethod: nearest\n");
        create_input_file(&dir, "reference", "ref.tif", b"reference raster data");
        create_input_file(&dir, "target", "tgt.tif", b"target raster data");

        let result_path = dir.path().join("reprojected.tif");
        fs::write(&result_path, b"reprojected output").unwrap();
        let rp = result_path.to_string_lossy().to_string();

        run_at(
            dir.path(),
            move |args: Args| -> std::result::Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
                let reference: RasterFile = args.input("reference")?;
                let target: RasterFile = args.input("target")?;
                let resolution: i64 = args.param("resolution")?;
                let method: String = args.param("method")?;
                assert!(reference.path.ends_with("ref.tif"));
                assert!(target.path.ends_with("tgt.tif"));
                assert_eq!(resolution, 10);
                assert_eq!(method, "nearest");
                Ok(RasterFile::new(rp))
            },
        )
        .unwrap();

        let output_files: Vec<_> = walkdir(dir.path().join("outputs"));
        assert_eq!(output_files.len(), 1);
        assert_eq!(fs::read(&output_files[0]).unwrap(), b"reprojected output");
    }

    fn walkdir(dir: std::path::PathBuf) -> Vec<std::path::PathBuf> {
        let mut files = Vec::new();
        if let Ok(entries) = fs::read_dir(&dir) {
            for entry in entries.flatten() {
                let path = entry.path();
                if path.is_file() {
                    files.push(path);
                } else if path.is_dir() {
                    files.extend(walkdir(path));
                }
            }
        }
        files
    }
}
