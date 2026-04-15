//! Block: `data.census_acs` — Census ACS Data API.
//!
//! The Census API requires a key. Pending real secret-management, the
//! block reads `CENSUS_API_KEY` from the process environment and errors
//! cleanly if it's absent.

use std::path::{Path, PathBuf};

use spade::{Args, TabularFile};

use crate::catalog::sources::census_acs_base_url;
use crate::common::error::{DataError, Result};

fn build_request_url(
    base: &str,
    year: i64,
    dataset: &str,
    variables: &str,
    geography: &str,
    key: &str,
) -> String {
    // Format: {base}/{year}/acs/{dataset}?get={vars}&for={geo}&key={key}
    // `geography` may contain an `in=` clause delimited by `&` — keep as-is.
    format!(
        "{base}/{year}/acs/{dataset}?get={vars}&for={geo}&key={key}",
        base = base.trim_end_matches('/'),
        vars = variables,
        geo = geography,
        key = key
    )
}

fn json_to_csv(body: &str) -> Result<String> {
    // The Census API returns a JSON array of arrays; the first row is
    // the header.
    let rows: Vec<Vec<String>> = serde_json::from_str(body)?;
    let mut out = String::new();
    for (i, row) in rows.iter().enumerate() {
        if i > 0 {
            out.push('\n');
        }
        for (j, cell) in row.iter().enumerate() {
            if j > 0 {
                out.push(',');
            }
            let needs_quote = cell.contains(',') || cell.contains('"') || cell.contains('\n');
            if needs_quote {
                out.push('"');
                for c in cell.chars() {
                    if c == '"' {
                        out.push('"');
                    }
                    out.push(c);
                }
                out.push('"');
            } else {
                out.push_str(cell);
            }
        }
    }
    Ok(out)
}

pub(crate) fn handler(args: Args, base: &Path) -> Result<TabularFile> {
    let year: i64 = args.param::<i64>("year").map_err(|_| DataError::BadArgument {
        name: "year".into(),
        reason: "required".into(),
    })?;
    let dataset: String = args
        .param::<String>("dataset")
        .map_err(|_| DataError::BadArgument {
            name: "dataset".into(),
            reason: "required".into(),
        })?;
    if dataset != "acs1" && dataset != "acs5" {
        return Err(DataError::BadArgument {
            name: "dataset".into(),
            reason: "must be 'acs1' or 'acs5'".into(),
        });
    }
    let table: String = args
        .param::<String>("table")
        .map_err(|_| DataError::BadArgument {
            name: "table".into(),
            reason: "required".into(),
        })?;
    let geography: String = args
        .param::<String>("geography")
        .map_err(|_| DataError::BadArgument {
            name: "geography".into(),
            reason: "required".into(),
        })?;

    let variables_raw = args.param::<String>("variables").unwrap_or_default();
    let variables = if variables_raw.trim().is_empty() {
        format!("NAME,{table}_001E")
    } else {
        variables_raw
    };

    let key = std::env::var("CENSUS_API_KEY").map_err(|_| DataError::BadArgument {
        name: "CENSUS_API_KEY".into(),
        reason:
            "CENSUS_API_KEY env var is required (pending proper secret-management integration)"
                .into(),
    })?;

    let url = build_request_url(
        &census_acs_base_url(),
        year,
        &dataset,
        &variables,
        &geography,
        &key,
    );

    let client = reqwest::blocking::Client::builder()
        .redirect(reqwest::redirect::Policy::limited(5))
        .build()?;
    let resp = client.get(&url).send()?;
    if resp.status() == reqwest::StatusCode::NOT_FOUND {
        return Err(DataError::NotFound(url.clone()));
    }
    if resp.status() == reqwest::StatusCode::FORBIDDEN
        || resp.status() == reqwest::StatusCode::UNAUTHORIZED
    {
        return Err(DataError::Unauthorized(url.clone()));
    }
    if !resp.status().is_success() {
        return Err(DataError::Network {
            uri: url,
            source: format!("HTTP {}", resp.status()).into(),
        });
    }
    let body = resp.text()?;

    let csv = json_to_csv(&body)?;
    let out_path = base.join("acs.csv");
    std::fs::write(&out_path, csv)?;
    Ok(TabularFile::new(out_path.to_string_lossy()))
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
    fn json_to_csv_simple() {
        let body = r#"[["NAME","B01003_001E","state"],["California","39000000","06"]]"#;
        let csv = json_to_csv(body).unwrap();
        assert_eq!(csv, "NAME,B01003_001E,state\nCalifornia,39000000,06");
    }

    #[test]
    fn json_to_csv_quotes_commas() {
        let body = r#"[["name"],["foo, bar"]]"#;
        let csv = json_to_csv(body).unwrap();
        assert_eq!(csv, "name\n\"foo, bar\"");
    }
}
