//! Data block collection — data-import plugins for the Spade pipeline runtime.
//!
//! This library exposes two layers:
//!
//! 1. **Generic OpenDAL blocks** — [`read`], [`read_collection`], [`write`],
//!    [`write_collection`], [`list`], [`stat`]. Users supply a URI; scheme
//!    dispatch selects the right backend.
//! 2. **Catalog blocks** — one module per well-known public dataset,
//!    under [`catalog`]. Each hides the backend and exposes only
//!    domain-relevant arguments.

pub mod common;

// Generic blocks
pub mod list;
pub mod read;
pub mod read_collection;
pub mod stat;
pub mod write;
pub mod write_collection;

// Catalog blocks
pub mod catalog;
