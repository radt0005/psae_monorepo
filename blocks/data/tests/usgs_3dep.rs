mod common;

use std::fs;
use std::sync::Mutex;

use data::catalog::usgs_3dep;

use crate::common::{put_input, work_dir, write_params};

static ENV_LOCK: Mutex<()> = Mutex::new(());

fn sample_aoi(dir: &std::path::Path) {
    let geojson = r#"{
        "type": "FeatureCollection",
        "features": [
            {"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[-120.0,35.0],[-119.0,35.0],[-119.0,36.0],[-120.0,36.0],[-120.0,35.0]]]},"properties":{}}
        ]
    }"#;
    put_input(dir, "aoi", "aoi.geojson", geojson.as_bytes());
}

#[tokio::test]
async fn fetches_two_tiles() {
    use wiremock::matchers::{method, path, query_param_contains};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    let api_body = serde_json::json!({
        "items": [
            {"title": "USGS 1m DEM Tile A", "downloadURL": format!("{}/tileA.tif", server.uri())},
            {"title": "USGS 1m DEM Tile B", "downloadURL": format!("{}/tileB.tif", server.uri())}
        ]
    });
    Mock::given(method("GET"))
        .and(path("/products"))
        .and(query_param_contains("bbox", "-120"))
        .respond_with(ResponseTemplate::new(200).set_body_json(api_body))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/tileA.tif"))
        .respond_with(ResponseTemplate::new(200).set_body_bytes(b"TIFA" as &[u8]))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/tileB.tif"))
        .respond_with(ResponseTemplate::new(200).set_body_bytes(b"TIFB" as &[u8]))
        .mount(&server)
        .await;
    let server_uri = server.uri();

    let handle = tokio::task::spawn_blocking(move || {
        let _g = ENV_LOCK.lock().unwrap();
        let dir = work_dir();
        sample_aoi(dir.path());
        unsafe {
            std::env::set_var("USGS_3DEP_BASE_URL", &server_uri);
        }
        write_params(dir.path(), "resolution: 1m\nproduct: DEM\n");
        let r = usgs_3dep::run_in(dir.path());
        unsafe {
            std::env::remove_var("USGS_3DEP_BASE_URL");
        }
        (dir, r)
    });
    let (dir, result) = handle.await.unwrap();
    result.unwrap();

    let out_dir = dir.path().join("outputs/rasters");
    let count = fs::read_dir(&out_dir).unwrap().count();
    assert_eq!(count, 2);
}

#[test]
fn bad_resolution_rejected() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    sample_aoi(dir.path());
    write_params(dir.path(), "resolution: 3m\nproduct: DEM\n");
    let err = usgs_3dep::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("resolution"));
}
