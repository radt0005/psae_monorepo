mod common;

use std::fs;
use std::sync::Mutex;

use data::catalog::osm_extract_shp;
use data::common::download::build_test_zip;

use crate::common::{work_dir, write_params};

static ENV_LOCK: Mutex<()> = Mutex::new(());

#[tokio::test]
async fn shp_happy_path() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    let zip = build_test_zip(&[("roads.shp", b"SHP"), ("roads.dbf", b"DBF")]).unwrap();
    Mock::given(method("GET"))
        .and(path("/north-america/us/oregon-latest-free.shp.zip"))
        .respond_with(ResponseTemplate::new(200).set_body_bytes(zip))
        .mount(&server)
        .await;
    let server_uri = server.uri();

    let handle = tokio::task::spawn_blocking(move || {
        let _g = ENV_LOCK.lock().unwrap();
        let dir = work_dir();
        unsafe {
            std::env::set_var("GEOFABRIK_BASE_URL", &server_uri);
        }
        write_params(dir.path(), "region: north-america/us/oregon\n");
        let r = osm_extract_shp::run_in(dir.path());
        unsafe {
            std::env::remove_var("GEOFABRIK_BASE_URL");
        }
        (dir, r)
    });
    let (dir, result) = handle.await.unwrap();
    result.unwrap();

    let out = dir.path().join("outputs/directory");
    assert!(out.exists());
    assert_eq!(fs::read_dir(&out).unwrap().count(), 2);
}

#[test]
fn rejects_path_traversal_shp() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(dir.path(), "region: ../evil\n");
    let err = osm_extract_shp::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().to_lowercase().contains("slug"));
}
