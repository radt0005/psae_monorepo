//! Block: `data.fia` — USFS Forest Inventory and Analysis (CSV).
//!
//! Downloads the USFS DataMart CSV archive for a given state (or the
//! whole country) and extracts matching tables. The whole archive is
//! fetched even when only a subset of tables is requested — USFS does
//! not offer per-table downloads, so this is the honest shape of the
//! upstream.

use std::path::{Path, PathBuf};

use spade::{Args, TabularFileCollection};

use crate::catalog::sources::fia_base_url;
use crate::common::download::{extract_zip, fetch_to};
use crate::common::error::{DataError, Result};
use crate::common::params::parse_csv_list;

const STATES: &[&str] = &[
    "AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE", "FL", "GA", "HI", "ID", "IL", "IN", "IA", "KS",
    "KY", "LA", "ME", "MD", "MA", "MI", "MN", "MS", "MO", "MT", "NE", "NV", "NH", "NJ", "NM", "NY",
    "NC", "ND", "OH", "OK", "OR", "PA", "RI", "SC", "SD", "TN", "TX", "UT", "VT", "VA", "WA", "WV",
    "WI", "WY", "DC", "PR", "VI", "GU", "AS", "MP",
];

fn validate_state(state: &str) -> Result<String> {
    let upper = state.to_ascii_uppercase();
    if upper == "ALL" {
        return Ok("ALL".to_string());
    }
    if STATES.contains(&upper.as_str()) {
        Ok(upper)
    } else {
        Err(DataError::BadArgument {
            name: "state".into(),
            reason: format!("unknown state code '{state}'"),
        })
    }
}

fn archive_filename(state: &str) -> String {
    match state {
        "ALL" => "ENTIRE_CSV.zip".into(),
        other => format!("{other}_CSV.zip"),
    }
}

fn matches_table(name: &str, state: &str, wanted: &[String]) -> bool {
    // FIA filenames look like "<STATE>_<TABLE>.csv" or "<TABLE>.csv" in
    // the national archive. Normalise to the table stem.
    let stem = match Path::new(name).file_stem().and_then(|s| s.to_str()) {
        Some(s) => s,
        None => return false,
    };
    let ext_ok = Path::new(name)
        .extension()
        .and_then(|s| s.to_str())
        .is_some_and(|e| e.eq_ignore_ascii_case("csv"));
    if !ext_ok {
        return false;
    }
    let table = if state != "ALL" {
        stem.strip_prefix(&format!("{state}_"))
            .unwrap_or(stem)
            .to_ascii_uppercase()
    } else {
        stem.to_ascii_uppercase()
    };
    if wanted.is_empty() {
        return true;
    }
    wanted.iter().any(|t| t.to_ascii_uppercase() == table)
}

pub(crate) fn handler(args: Args, base: &Path) -> Result<TabularFileCollection> {
    let state_raw: String = args
        .param::<String>("state")
        .unwrap_or_else(|_| "all".to_string());
    let state = validate_state(&state_raw)?;

    let tables_raw: String = args
        .param::<String>("tables")
        .unwrap_or_default();
    let wanted = parse_csv_list(&tables_raw);

    let url = format!("{}/{}", fia_base_url(), archive_filename(&state));

    let tmp = tempfile::tempdir()?;
    let zip_path = tmp.path().join("fia.zip");
    fetch_to(&zip_path, &url)?;

    let out_dir = base.join("tables");
    let state_clone = state.clone();
    let wanted_clone = wanted.clone();
    let extracted = extract_zip(&zip_path, &out_dir, |n| {
        matches_table(n, &state_clone, &wanted_clone)
    })?;

    if !wanted.is_empty() && extracted.is_empty() {
        return Err(DataError::BadArgument {
            name: "tables".into(),
            reason: "no tables matched after extraction".into(),
        });
    }

    let paths: Vec<String> = extracted
        .into_iter()
        .map(|p| p.to_string_lossy().to_string())
        .collect();
    Ok(TabularFileCollection::new(paths))
}

pub fn entry() {
    let base = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
    spade::run(move |args| handler(args, &base).map_err(Into::into));
}

pub fn run_in(base: &Path) -> Result<()> {
    let args = spade::scanning::build_args_from(base)?;
    let out = handler(args, base)?;
    spade::output::write_outputs_to(out, base, None)?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn validates_state() {
        assert_eq!(validate_state("ny").unwrap(), "NY");
        assert_eq!(validate_state("ALL").unwrap(), "ALL");
        assert!(validate_state("ZZ").is_err());
    }

    #[test]
    fn archive_filename_branches() {
        assert_eq!(archive_filename("ALL"), "ENTIRE_CSV.zip");
        assert_eq!(archive_filename("NY"), "NY_CSV.zip");
    }

    #[test]
    fn matches_table_state_scoped() {
        let wanted = vec!["PLOT".to_string(), "TREE".to_string()];
        assert!(matches_table("NY_PLOT.csv", "NY", &wanted));
        assert!(matches_table("NY_TREE.csv", "NY", &wanted));
        assert!(!matches_table("NY_COND.csv", "NY", &wanted));
        assert!(!matches_table("NY_PLOT.txt", "NY", &wanted));
    }

    #[test]
    fn matches_table_empty_wanted_matches_all_csv() {
        assert!(matches_table("NY_ANY.csv", "NY", &[]));
        assert!(!matches_table("NY_ANY.xml", "NY", &[]));
    }
}
