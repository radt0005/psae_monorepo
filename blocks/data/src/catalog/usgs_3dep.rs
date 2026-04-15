//! Block: `data.usgs_3dep` — USGS 3D Elevation Program tile fetch.

use std::path::{Path, PathBuf};

use serde_json::Value;
use spade::{Args, RasterFileCollection, VectorFile};

use crate::catalog::sources::usgs_3dep_api_base_url;
use crate::common::download::fetch_to;
use crate::common::error::{DataError, Result};

/// Extract a bounding box `[min_x, min_y, max_x, max_y]` from an arbitrary
/// GeoJSON document.
fn bbox_from_geojson(v: &Value) -> Option<[f64; 4]> {
    if let Some(arr) = v.get("bbox").and_then(|b| b.as_array()) {
        if arr.len() >= 4 {
            let mut out = [0.0; 4];
            for (i, x) in out.iter_mut().enumerate() {
                *x = arr.get(i)?.as_f64()?;
            }
            return Some(out);
        }
    }
    // Otherwise walk coordinates.
    let mut min_x = f64::INFINITY;
    let mut min_y = f64::INFINITY;
    let mut max_x = f64::NEG_INFINITY;
    let mut max_y = f64::NEG_INFINITY;
    walk_coords(v, &mut |x, y| {
        if x < min_x {
            min_x = x;
        }
        if y < min_y {
            min_y = y;
        }
        if x > max_x {
            max_x = x;
        }
        if y > max_y {
            max_y = y;
        }
    });
    if min_x.is_finite() && max_x.is_finite() {
        Some([min_x, min_y, max_x, max_y])
    } else {
        None
    }
}

fn walk_coords<F: FnMut(f64, f64)>(v: &Value, f: &mut F) {
    match v {
        Value::Array(arr) => {
            // A coordinate pair/triple is an array of 2+ numbers where
            // the first two are both numbers.
            if arr.len() >= 2 && arr[0].is_number() && arr[1].is_number() {
                if let (Some(x), Some(y)) = (arr[0].as_f64(), arr[1].as_f64()) {
                    f(x, y);
                    return;
                }
            }
            for child in arr {
                walk_coords(child, f);
            }
        }
        Value::Object(map) => {
            for (_, child) in map {
                walk_coords(child, f);
            }
        }
        _ => {}
    }
}

pub(crate) fn handler(args: Args, base: &Path) -> Result<RasterFileCollection> {
    let aoi: VectorFile = args.input("aoi")?;
    let aoi_text = std::fs::read_to_string(&aoi.path)?;
    let aoi_value: Value = serde_json::from_str(&aoi_text)?;
    let bbox = bbox_from_geojson(&aoi_value).ok_or_else(|| DataError::BadArgument {
        name: "aoi".into(),
        reason: "could not derive a bbox from AOI GeoJSON".into(),
    })?;

    let resolution: String = args
        .param::<String>("resolution")
        .map_err(|_| DataError::BadArgument {
            name: "resolution".into(),
            reason: "required".into(),
        })?;
    if !matches!(resolution.as_str(), "1m" | "10m" | "30m" | "60m") {
        return Err(DataError::BadArgument {
            name: "resolution".into(),
            reason: format!("unknown resolution '{resolution}'"),
        });
    }
    let product: String = args
        .param::<String>("product")
        .unwrap_or_else(|_| "DEM".to_string());

    let api_url = format!(
        "{}/products?bbox={},{},{},{}&datasets=National%20Elevation%20Dataset%20(NED)%20{resolution}",
        usgs_3dep_api_base_url(),
        bbox[0],
        bbox[1],
        bbox[2],
        bbox[3],
    );

    let client = reqwest::blocking::Client::builder()
        .redirect(reqwest::redirect::Policy::limited(5))
        .build()?;
    let resp = client.get(&api_url).send()?;
    if !resp.status().is_success() {
        return Err(DataError::Network {
            uri: api_url,
            source: format!("HTTP {}", resp.status()).into(),
        });
    }
    let body: Value = resp.json()?;
    let items = body.get("items").and_then(|v| v.as_array()).ok_or_else(|| {
        DataError::BadArgument {
            name: "response".into(),
            reason: "expected `items` array in TNM response".into(),
        }
    })?;

    let mut filtered: Vec<&Value> = items
        .iter()
        .filter(|i| {
            let matches_product = i
                .get("title")
                .and_then(|v| v.as_str())
                .map(|s| s.contains(&product))
                .unwrap_or(true);
            let matches_res = i
                .get("title")
                .and_then(|v| v.as_str())
                .map(|s| s.contains(&resolution))
                .unwrap_or(true);
            matches_product && matches_res
        })
        .collect();
    // Stable ordering by title so tests are deterministic.
    filtered.sort_by(|a, b| {
        a.get("title")
            .and_then(|v| v.as_str())
            .unwrap_or("")
            .cmp(b.get("title").and_then(|v| v.as_str()).unwrap_or(""))
    });

    let out_dir = base.join("tiles");
    std::fs::create_dir_all(&out_dir)?;

    let mut paths = Vec::with_capacity(filtered.len());
    for item in filtered {
        let url = item
            .get("downloadURL")
            .and_then(|v| v.as_str())
            .ok_or_else(|| DataError::BadArgument {
                name: "response".into(),
                reason: "item missing downloadURL".into(),
            })?;
        let filename = Path::new(url)
            .file_name()
            .and_then(|s| s.to_str())
            .unwrap_or("tile.tif")
            .to_string();
        let dest = out_dir.join(&filename);
        fetch_to(&dest, url)?;
        paths.push(dest.to_string_lossy().to_string());
    }
    Ok(RasterFileCollection::new(paths))
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
    fn bbox_from_feature_polygon() {
        let fc = serde_json::json!({
            "type": "FeatureCollection",
            "features": [
                {"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[0.0,0.0],[2.0,0.0],[2.0,1.0],[0.0,1.0],[0.0,0.0]]]},"properties":{}}
            ]
        });
        let bbox = bbox_from_geojson(&fc).unwrap();
        assert_eq!(bbox, [0.0, 0.0, 2.0, 1.0]);
    }

    #[test]
    fn bbox_honoured_if_present() {
        let fc = serde_json::json!({ "type": "Feature", "bbox": [1.0, 2.0, 3.0, 4.0] });
        let bbox = bbox_from_geojson(&fc).unwrap();
        assert_eq!(bbox, [1.0, 2.0, 3.0, 4.0]);
    }
}
