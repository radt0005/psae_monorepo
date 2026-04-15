mod common;

use std::fs;
use std::sync::Mutex;

use data::catalog::nhd;
use data::common::download::build_test_zip;

use crate::common::{work_dir, write_params};

static ENV_LOCK: Mutex<()> = Mutex::new(());

#[tokio::test]
async fn nhd_huc4_happy_path() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    let zip = build_test_zip(&[("NHD_H_0102_HU4.gpkg", b"GPKG")]).unwrap();
    Mock::given(method("GET"))
        .and(path("/HU4/GPKG/NHD_H_0102_HU4_GPKG.zip"))
        .respond_with(ResponseTemplate::new(200).set_body_bytes(zip))
        .mount(&server)
        .await;
    let server_uri = server.uri();

    let handle = tokio::task::spawn_blocking(move || {
        let _g = ENV_LOCK.lock().unwrap();
        let dir = work_dir();
        unsafe {
            std::env::set_var("NHD_BASE_URL", &server_uri);
        }
        write_params(dir.path(), "huc: '0102'\nresolution: high\n");
        let r = nhd::run_in(dir.path());
        unsafe {
            std::env::remove_var("NHD_BASE_URL");
        }
        (dir, r)
    });
    let (dir, result) = handle.await.unwrap();
    result.unwrap();

    let out = dir.path().join("outputs/directory/NHD_H_0102_HU4.gpkg");
    assert!(out.exists(), "{:?}", out);
    assert_eq!(fs::read(&out).unwrap(), b"GPKG");
}

#[test]
fn huc_non_numeric_rejected() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(dir.path(), "huc: ABCD\nresolution: high\n");
    let err = nhd::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().to_lowercase().contains("numeric"));
}

#[test]
fn huc_wrong_length_rejected() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(dir.path(), "huc: '123'\nresolution: high\n");
    let err = nhd::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("4 or 8"));
}
