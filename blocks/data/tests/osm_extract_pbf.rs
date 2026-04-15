mod common;

use std::fs;
use std::sync::Mutex;

use data::catalog::osm_extract_pbf;

use crate::common::{work_dir, write_params};

static ENV_LOCK: Mutex<()> = Mutex::new(());

#[tokio::test]
async fn pbf_happy_path() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/north-america/us/oregon-latest.osm.pbf"))
        .respond_with(ResponseTemplate::new(200).set_body_bytes(b"PBF-BYTES" as &[u8]))
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
        let r = osm_extract_pbf::run_in(dir.path());
        unsafe {
            std::env::remove_var("GEOFABRIK_BASE_URL");
        }
        (dir, r)
    });
    let (dir, result) = handle.await.unwrap();
    result.unwrap();

    let out = dir.path().join("outputs/file/oregon-latest.osm.pbf");
    assert!(out.exists());
    assert_eq!(fs::read(&out).unwrap(), b"PBF-BYTES");
}

#[tokio::test]
async fn pbf_404() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/unknown-latest.osm.pbf"))
        .respond_with(ResponseTemplate::new(404))
        .mount(&server)
        .await;
    let server_uri = server.uri();

    let handle = tokio::task::spawn_blocking(move || {
        let _g = ENV_LOCK.lock().unwrap();
        let dir = work_dir();
        unsafe {
            std::env::set_var("GEOFABRIK_BASE_URL", &server_uri);
        }
        write_params(dir.path(), "region: unknown\n");
        let r = osm_extract_pbf::run_in(dir.path());
        unsafe {
            std::env::remove_var("GEOFABRIK_BASE_URL");
        }
        (dir, r)
    });
    let (_dir, result) = handle.await.unwrap();
    let err = result.unwrap_err();
    assert!(err.to_string().to_lowercase().contains("not found"));
}

#[test]
fn rejects_path_traversal() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(dir.path(), "region: ../evil\n");
    let err = osm_extract_pbf::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().to_lowercase().contains("slug"));
}
