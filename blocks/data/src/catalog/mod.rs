//! Catalog blocks — one module per well-known public dataset.
//!
//! Each catalog block hides its backend and exposes only
//! domain-relevant arguments. Upstream URL patterns live in
//! [`sources`] so corrections land in a single file.

pub mod sources;

pub mod census_acs;
pub mod census_tiger;
pub mod fia;
pub mod naturalearth_raster;
pub mod naturalearth_vector;
pub mod nhd;
pub mod nlcd;
pub mod osm_extract_pbf;
pub mod osm_extract_shp;
pub mod prism;
pub mod ssurgo;
pub mod usgs_3dep;
