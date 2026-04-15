//! Block: `data.nlcd` — National Land Cover Database.

use std::path::{Path, PathBuf};

use spade::{Args, RasterFile};

use crate::catalog::sources::nlcd_base_url;
use crate::common::download::{extract_zip_tree, fetch_to};
use crate::common::error::{DataError, Result};

fn validate_year(year: i64) -> Result<i64> {
    let accepted = [2001, 2004, 2006, 2008, 2011, 2013, 2016, 2019, 2021];
    if accepted.contains(&year) {
        Ok(year)
    } else {
        Err(DataError::BadArgument {
            name: "year".into(),
            reason: format!("NLCD is not published for year {year}"),
        })
    }
}

fn validate_product(product: &str) -> Result<&'static str> {
    Ok(match product {
        "land_cover" => "land_cover",
        "impervious" => "impervious",
        "canopy" => "canopy",
        other => {
            return Err(DataError::BadArgument {
                name: "product".into(),
                reason: format!("unknown product '{other}'"),
            })
        }
    })
}

fn validate_region(region: &str) -> Result<&'static str> {
    Ok(match region {
        "CONUS" => "CONUS",
        "AK" => "AK",
        "HI" => "HI",
        "PR" => "PR",
        other => {
            return Err(DataError::BadArgument {
                name: "region".into(),
                reason: format!("unknown region '{other}'"),
            })
        }
    })
}

fn filename_for(year: i64, product: &str, region: &str) -> String {
    format!("nlcd_{year}_{product}_{region}.zip")
}

pub(crate) fn handler(args: Args, base: &Path) -> Result<RasterFile> {
    let year = validate_year(args.param::<i64>("year").map_err(|_| DataError::BadArgument {
        name: "year".into(),
        reason: "required".into(),
    })?)?;
    let product = validate_product(&args.param::<String>("product").map_err(|_| {
        DataError::BadArgument {
            name: "product".into(),
            reason: "required".into(),
        }
    })?)?;
    let region_raw: String = args
        .param::<String>("region")
        .unwrap_or_else(|_| "CONUS".to_string());
    let region = validate_region(&region_raw)?;

    if !args.has_input("aoi") {
        eprintln!(
            "warning: no AOI supplied; NLCD {region} rasters can be > 5 GB. Consider clipping downstream with gdal.clip_raster_by_vector."
        );
    }

    let url = format!(
        "{}/{}",
        nlcd_base_url(),
        filename_for(year, product, region)
    );

    let tmp = tempfile::tempdir()?;
    let zip_path = tmp.path().join("nlcd.zip");
    fetch_to(&zip_path, &url)?;

    let out_dir = base.join("nlcd");
    let files = extract_zip_tree(&zip_path, &out_dir)?;
    let tif = files
        .iter()
        .find(|p| {
            matches!(
                p.extension().and_then(|s| s.to_str()),
                Some("tif") | Some("tiff") | Some("TIF") | Some("TIFF")
            )
        })
        .cloned()
        .or_else(|| files.first().cloned())
        .ok_or_else(|| DataError::BadArgument {
            name: "response".into(),
            reason: "archive contained no files".into(),
        })?;
    Ok(RasterFile::new(tif.to_string_lossy()))
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
