//! Block: `data.prism` — PRISM climate rasters (BIL format).

use std::path::{Path, PathBuf};

use spade::{Args, RasterFileCollection};

use crate::catalog::sources::prism_base_url;
use crate::common::download::{extract_zip_tree, fetch_to};
use crate::common::error::{DataError, Result};

fn parse_date(s: &str) -> Result<(i32, u32, u32)> {
    let parts: Vec<&str> = s.split('-').collect();
    if parts.len() != 3 {
        return Err(DataError::BadArgument {
            name: "date".into(),
            reason: format!("expected YYYY-MM-DD (got '{s}')"),
        });
    }
    let year: i32 = parts[0].parse().map_err(|_| DataError::BadArgument {
        name: "date".into(),
        reason: format!("bad year in '{s}'"),
    })?;
    let month: u32 = parts[1].parse().map_err(|_| DataError::BadArgument {
        name: "date".into(),
        reason: format!("bad month in '{s}'"),
    })?;
    let day: u32 = parts[2].parse().map_err(|_| DataError::BadArgument {
        name: "date".into(),
        reason: format!("bad day in '{s}'"),
    })?;
    if !(1..=12).contains(&month) || !(1..=31).contains(&day) {
        return Err(DataError::BadArgument {
            name: "date".into(),
            reason: format!("out-of-range month/day in '{s}'"),
        });
    }
    Ok((year, month, day))
}

fn enumerate_periods(
    start: (i32, u32, u32),
    end: (i32, u32, u32),
    cadence: &str,
) -> Result<Vec<String>> {
    match cadence {
        "annual" => {
            let mut out = Vec::new();
            for y in start.0..=end.0 {
                out.push(format!("{y}"));
            }
            Ok(out)
        }
        "monthly" => {
            let mut out = Vec::new();
            let (sy, sm) = (start.0, start.1);
            let (ey, em) = (end.0, end.1);
            let mut y = sy;
            let mut m = sm;
            loop {
                out.push(format!("{y}{:02}", m));
                if y == ey && m == em {
                    break;
                }
                m += 1;
                if m > 12 {
                    m = 1;
                    y += 1;
                }
                if y > ey || (y == ey && m > em) {
                    break;
                }
            }
            Ok(out)
        }
        "daily" => {
            // Naive date increment with 31/30/28/29 month lengths.
            fn days_in(y: i32, m: u32) -> u32 {
                match m {
                    1 | 3 | 5 | 7 | 8 | 10 | 12 => 31,
                    4 | 6 | 9 | 11 => 30,
                    2 => {
                        let leap = (y % 4 == 0 && y % 100 != 0) || y % 400 == 0;
                        if leap {
                            29
                        } else {
                            28
                        }
                    }
                    _ => 0,
                }
            }
            let (mut y, mut m, mut d) = start;
            let (ey, em, ed) = end;
            let mut out = Vec::new();
            loop {
                out.push(format!("{y}{:02}{:02}", m, d));
                if y == ey && m == em && d == ed {
                    break;
                }
                d += 1;
                if d > days_in(y, m) {
                    d = 1;
                    m += 1;
                    if m > 12 {
                        m = 1;
                        y += 1;
                    }
                }
                if y > ey || (y == ey && m > em) || (y == ey && m == em && d > ed) {
                    break;
                }
            }
            Ok(out)
        }
        other => Err(DataError::BadArgument {
            name: "cadence".into(),
            reason: format!("unknown cadence '{other}'"),
        }),
    }
}

pub(crate) fn handler(args: Args, base: &Path) -> Result<RasterFileCollection> {
    let variable: String = args.param::<String>("variable").map_err(|_| {
        DataError::BadArgument {
            name: "variable".into(),
            reason: "required".into(),
        }
    })?;
    if !matches!(
        variable.as_str(),
        "ppt" | "tmean" | "tmin" | "tmax" | "tdmean" | "vpdmin" | "vpdmax"
    ) {
        return Err(DataError::BadArgument {
            name: "variable".into(),
            reason: format!("unknown variable '{variable}'"),
        });
    }
    let start_raw: String = args
        .param::<String>("start")
        .map_err(|_| DataError::BadArgument {
            name: "start".into(),
            reason: "required".into(),
        })?;
    let end_raw: String = args
        .param::<String>("end")
        .map_err(|_| DataError::BadArgument {
            name: "end".into(),
            reason: "required".into(),
        })?;
    let start = parse_date(&start_raw)?;
    let end = parse_date(&end_raw)?;

    let resolution: String = args
        .param::<String>("resolution")
        .unwrap_or_else(|_| "4km".to_string());
    if resolution == "800m" {
        return Err(DataError::BadArgument {
            name: "resolution".into(),
            reason: "800m PRISM data is paywalled and not supported by this block".into(),
        });
    }
    if resolution != "4km" {
        return Err(DataError::BadArgument {
            name: "resolution".into(),
            reason: format!("unknown resolution '{resolution}'"),
        });
    }

    let cadence: String = args
        .param::<String>("cadence")
        .unwrap_or_else(|_| "monthly".to_string());
    let periods = enumerate_periods(start, end, &cadence)?;

    let out_dir = base.join("rasters");
    std::fs::create_dir_all(&out_dir)?;
    let mut out_paths = Vec::with_capacity(periods.len());

    for period in periods {
        let url = format!("{}/{}/{}/{}", prism_base_url(), variable, cadence, period);
        let tmp = tempfile::tempdir()?;
        let zip_path = tmp.path().join("prism.zip");
        fetch_to(&zip_path, &url)?;
        let target = out_dir.join(&period);
        let extracted = extract_zip_tree(&zip_path, &target)?;
        // Prefer a .bil as the "main" raster file.
        let bil = extracted
            .iter()
            .find(|p| {
                p.extension()
                    .and_then(|s| s.to_str())
                    .map(|e| e.eq_ignore_ascii_case("bil"))
                    .unwrap_or(false)
            })
            .or_else(|| extracted.first())
            .cloned();
        if let Some(p) = bil {
            out_paths.push(p.to_string_lossy().to_string());
        }
    }
    Ok(RasterFileCollection::new(out_paths))
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
    fn enumerate_monthly_single_month() {
        let v = enumerate_periods((2023, 3, 1), (2023, 3, 1), "monthly").unwrap();
        assert_eq!(v, vec!["202303".to_string()]);
    }

    #[test]
    fn enumerate_monthly_span() {
        let v = enumerate_periods((2023, 11, 1), (2024, 2, 1), "monthly").unwrap();
        assert_eq!(v, vec!["202311", "202312", "202401", "202402"]);
    }

    #[test]
    fn enumerate_daily_cross_month() {
        let v = enumerate_periods((2023, 1, 30), (2023, 2, 2), "daily").unwrap();
        assert_eq!(v, vec!["20230130", "20230131", "20230201", "20230202"]);
    }

    #[test]
    fn enumerate_annual() {
        let v = enumerate_periods((2020, 1, 1), (2022, 1, 1), "annual").unwrap();
        assert_eq!(v, vec!["2020", "2021", "2022"]);
    }
}
