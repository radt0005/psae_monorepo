//! Spade runtime library for building blocks in Rust.
//!
//! This library provides types and functions for writing Spade blocks. Block authors
//! define a handler function and pass it to [`run()`], which handles loading inputs,
//! parameters, calling the handler, and writing outputs.
//!
//! # Quick Start
//!
//! ```ignore
//! use spade::{run, Args, RasterFile};
//!
//! fn handler(args: Args) -> Result<RasterFile, Box<dyn std::error::Error + Send + Sync>> {
//!     let source: RasterFile = args.input("source")?;
//!     let resolution: f64 = args.param("resolution")?;
//!     // process the raster...
//!     Ok(RasterFile::new("result.tif"))
//! }
//!
//! fn main() {
//!     run(handler);
//! }
//! ```
//!
//! # Multiple Outputs
//!
//! ```ignore
//! use spade::{run, Args, Outputs, RasterFile, JsonFile};
//!
//! fn handler(args: Args) -> Result<Outputs, Box<dyn std::error::Error + Send + Sync>> {
//!     let source: RasterFile = args.input("source")?;
//!     let mut outputs = Outputs::new();
//!     outputs.add("raster", RasterFile::new("result.tif"));
//!     outputs.add("stats", JsonFile::new("stats.json"));
//!     Ok(outputs)
//! }
//!
//! fn main() {
//!     run(handler);
//! }
//! ```

pub mod error;
pub mod types;
pub mod scanning;
pub mod output;
pub mod run;
pub mod build;

// Re-export public API
pub use error::{SpadeError, Result};
pub use types::{
    File, Directory, RasterFile, VectorFile, TabularFile, JsonFile,
    FileCollection, RasterFileCollection, VectorFileCollection, TabularFileCollection,
    SpadeType, FromInput, IntoOutput,
};
pub use scanning::Args;
pub use output::Outputs;
pub use run::run;
pub use build::{build, ManifestBuilder};
