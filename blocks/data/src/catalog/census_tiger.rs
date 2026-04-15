//! Block: `data.census_tiger` — Census TIGER/Line shapefiles.

use std::path::{Path, PathBuf};

use spade::{Args, Directory};

use crate::catalog::sources::census_tiger_base_url;
use crate::common::download::{extract_zip_tree, fetch_to};
use crate::common::error::{DataError, Result};

/// Two-letter USPS → 2-digit FIPS for the common state codes.
fn state_fips(code: &str) -> Option<&'static str> {
    Some(match code.to_ascii_uppercase().as_str() {
        "AL" => "01",
        "AK" => "02",
        "AZ" => "04",
        "AR" => "05",
        "CA" => "06",
        "CO" => "08",
        "CT" => "09",
        "DE" => "10",
        "DC" => "11",
        "FL" => "12",
        "GA" => "13",
        "HI" => "15",
        "ID" => "16",
        "IL" => "17",
        "IN" => "18",
        "IA" => "19",
        "KS" => "20",
        "KY" => "21",
        "LA" => "22",
        "ME" => "23",
        "MD" => "24",
        "MA" => "25",
        "MI" => "26",
        "MN" => "27",
        "MS" => "28",
        "MO" => "29",
        "MT" => "30",
        "NE" => "31",
        "NV" => "32",
        "NH" => "33",
        "NJ" => "34",
        "NM" => "35",
        "NY" => "36",
        "NC" => "37",
        "ND" => "38",
        "OH" => "39",
        "OK" => "40",
        "OR" => "41",
        "PA" => "42",
        "RI" => "44",
        "SC" => "45",
        "SD" => "46",
        "TN" => "47",
        "TX" => "48",
        "UT" => "49",
        "VT" => "50",
        "VA" => "51",
        "WA" => "53",
        "WV" => "54",
        "WI" => "55",
        "WY" => "56",
        "PR" => "72",
        _ => return None,
    })
}

#[derive(Debug, Clone, Copy)]
enum LayerScope {
    National,
    State,
}

fn layer_info(layer: &str) -> Result<(LayerScope, &'static str)> {
    // Returns (scope, URL-tail template with {year} and {fips}
    // placeholders). The TIGER URL is `<base>/TIGER<YEAR>/<tail>.zip`.
    Ok(match layer.to_ascii_lowercase().as_str() {
        "states" => (LayerScope::National, "STATE/tl_{year}_us_state"),
        "counties" => (LayerScope::National, "COUNTY/tl_{year}_us_county"),
        "tracts" => (LayerScope::State, "TRACT/tl_{year}_{fips}_tract"),
        "block_groups" => (LayerScope::State, "BG/tl_{year}_{fips}_bg"),
        "blocks" => (LayerScope::State, "TABBLOCK20/tl_{year}_{fips}_tabblock20"),
        "zcta" => (LayerScope::National, "ZCTA520/tl_{year}_us_zcta520"),
        "roads" => (LayerScope::State, "ROADS/tl_{year}_{fips}_roads"),
        "primary_roads" => (
            LayerScope::National,
            "PRIMARYROADS/tl_{year}_us_primaryroads",
        ),
        "primary_secondary_roads" => (
            LayerScope::State,
            "PRISECROADS/tl_{year}_{fips}_prisecroads",
        ),
        "rails" => (LayerScope::National, "RAILS/tl_{year}_us_rails"),
        "places" => (LayerScope::State, "PLACE/tl_{year}_{fips}_place"),
        "urban_areas" => (LayerScope::National, "UAC20/tl_{year}_us_uac20"),
        "cousub" => (LayerScope::State, "COUSUB/tl_{year}_{fips}_cousub"),
        other => {
            return Err(DataError::BadArgument {
                name: "layer".into(),
                reason: format!("unknown layer '{other}'"),
            })
        }
    })
}

pub(crate) fn handler(args: Args, base: &Path) -> Result<Directory> {
    let year: i64 = args
        .param::<i64>("year")
        .map_err(|_| DataError::BadArgument {
            name: "year".into(),
            reason: "required".into(),
        })?;
    if !(2010..=2100).contains(&year) {
        return Err(DataError::BadArgument {
            name: "year".into(),
            reason: format!("year {year} is out of supported range"),
        });
    }

    let layer: String = args
        .param::<String>("layer")
        .map_err(|_| DataError::BadArgument {
            name: "layer".into(),
            reason: "required".into(),
        })?;
    let (scope, pattern) = layer_info(&layer)?;

    let state_raw = args.param::<String>("state").ok().filter(|s| !s.is_empty());
    let fips = match scope {
        LayerScope::National => None,
        LayerScope::State => {
            let raw = state_raw.ok_or_else(|| DataError::BadArgument {
                name: "state".into(),
                reason: format!("layer '{layer}' requires a state"),
            })?;
            let code = if raw.chars().all(|c| c.is_ascii_digit()) {
                if raw.len() != 2 {
                    return Err(DataError::BadArgument {
                        name: "state".into(),
                        reason: "FIPS must be 2 digits".into(),
                    });
                }
                raw
            } else {
                state_fips(&raw)
                    .ok_or_else(|| DataError::BadArgument {
                        name: "state".into(),
                        reason: format!("unknown state '{raw}'"),
                    })?
                    .to_string()
            };
            Some(code)
        }
    };

    let tail = pattern
        .replace("{year}", &year.to_string())
        .replace("{fips}", fips.as_deref().unwrap_or(""));
    let url = format!("{}/TIGER{year}/{tail}.zip", census_tiger_base_url());

    let tmp = tempfile::tempdir()?;
    let zip_path = tmp.path().join("tiger.zip");
    fetch_to(&zip_path, &url)?;

    let out_dir = base.join("shapefile");
    extract_zip_tree(&zip_path, &out_dir)?;

    Ok(Directory::new(out_dir.to_string_lossy()))
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
    fn fips_lookup() {
        assert_eq!(state_fips("NY"), Some("36"));
        assert_eq!(state_fips("ca"), Some("06"));
        assert_eq!(state_fips("ZZ"), None);
    }

    #[test]
    fn layer_dispatch() {
        let (scope, _) = layer_info("states").unwrap();
        assert!(matches!(scope, LayerScope::National));
        let (scope, _) = layer_info("tracts").unwrap();
        assert!(matches!(scope, LayerScope::State));
        assert!(layer_info("nonsense").is_err());
    }
}
