//! Upstream URL patterns for catalog blocks.
//!
//! Each dataset gets a `*_base_url()` function that returns the root of
//! its upstream distribution. Tests override the real URL via a
//! per-block environment variable (e.g. `FIA_BASE_URL`); production
//! code falls back to the published constant.
//!
//! Keeping the URLs centralised means the occasional USFS / MRLC /
//! Geofabrik path restructure is a single-file change.

/// Helper: read an override from an env var, otherwise fall back to the
/// compile-time default.
fn env_or(var: &str, default: &str) -> String {
    std::env::var(var).unwrap_or_else(|_| default.to_string())
}

pub fn fia_base_url() -> String {
    env_or(
        "FIA_BASE_URL",
        "https://apps.fs.usda.gov/fia/datamart/CSV",
    )
}

pub fn census_tiger_base_url() -> String {
    env_or("CENSUS_TIGER_BASE_URL", "https://www2.census.gov/geo/tiger")
}

pub fn census_acs_base_url() -> String {
    env_or("CENSUS_ACS_BASE_URL", "https://api.census.gov/data")
}

pub fn naturalearth_base_url() -> String {
    env_or("NATURALEARTH_BASE_URL", "https://naciscdn.org/naturalearth")
}

pub fn usgs_3dep_api_base_url() -> String {
    env_or(
        "USGS_3DEP_BASE_URL",
        "https://tnmaccess.nationalmap.gov/api/v1",
    )
}

pub fn nlcd_base_url() -> String {
    env_or(
        "NLCD_BASE_URL",
        "https://www.mrlc.gov/downloads/sciweb1/shared/mrlc/downloads",
    )
}

pub fn nhd_base_url() -> String {
    env_or(
        "NHD_BASE_URL",
        "https://prd-tnm.s3.amazonaws.com/StagedProducts/Hydrography/NHD",
    )
}

pub fn ssurgo_base_url() -> String {
    env_or(
        "SSURGO_BASE_URL",
        "https://websoilsurvey.nrcs.usda.gov/DSD/Download/Cache/SSA",
    )
}

pub fn prism_base_url() -> String {
    env_or("PRISM_BASE_URL", "https://services.nacse.org/prism/data/public")
}

pub fn geofabrik_base_url() -> String {
    env_or("GEOFABRIK_BASE_URL", "https://download.geofabrik.de")
}
