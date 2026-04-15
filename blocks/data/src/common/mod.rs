//! Shared helpers for the `data` block collection.
//!
//! Each block in this collection:
//!
//! 1. Accepts a URI or opaque domain arguments,
//! 2. Dispatches to an OpenDAL backend via [`backend::build_operator`]
//!    (for generic blocks) or hits an HTTP endpoint via [`download`]
//!    (for catalog blocks),
//! 3. Writes the result under the block's working directory and returns
//!    a Spade type describing the output.
//!
//! The `handler_entry` helper wraps a handler `FnOnce(Args) -> Result<T>`
//! so each block module only has to supply the handler logic; the
//! `entry()` functions in each block do `spade::run(move |args|
//! handler(args, &base).map_err(Into::into))`.

pub mod backend;
pub mod download;
pub mod error;
pub mod params;
pub mod runtime;
pub mod uri;

pub use error::{DataError, Result};

/// Bridge a handler `Result` into the `Box<dyn Error>` shape `spade::run`
/// expects.
pub fn handler_entry<F, T>(f: F)
where
    F: FnOnce(spade::Args) -> Result<T>,
    T: spade::IntoOutput + spade::SpadeType + 'static,
{
    spade::run(move |args| f(args).map_err(Into::into));
}
