mod common;

use std::fs;
use std::sync::Mutex;

use data::catalog::naturalearth_raster;
use data::common::download::build_test_zip;

use crate::common::{work_dir, write_params};

static ENV_LOCK: Mutex<()> = Mutex::new(());

#[tokio::test]
async fn happy_path_raster_10m() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    let zip = build_test_zip(&[("HYP_HR_SR_OB_DR.tif", b"TIFF")]).unwrap();
    Mock::given(method("GET"))
        .and(path("/10m/raster/HYP_HR_SR_OB_DR.zip"))
        .respond_with(ResponseTemplate::new(200).set_body_bytes(zip))
        .mount(&server)
        .await;
    let server_uri = server.uri();

    let handle = tokio::task::spawn_blocking(move || {
        let _g = ENV_LOCK.lock().unwrap();
        let dir = work_dir();
        unsafe {
            std::env::set_var("NATURALEARTH_BASE_URL", &server_uri);
        }
        write_params(dir.path(), "scale: 10m\ntheme: HYP_HR_SR_OB_DR\n");
        let r = naturalearth_raster::run_in(dir.path());
        unsafe {
            std::env::remove_var("NATURALEARTH_BASE_URL");
        }
        (dir, r)
    });
    let (dir, result) = handle.await.unwrap();
    result.unwrap();

    let out = dir.path().join("outputs/raster/HYP_HR_SR_OB_DR.tif");
    assert!(out.exists(), "expected {:?}", out);
    assert_eq!(fs::read(&out).unwrap(), b"TIFF");
}

#[test]
fn raster_rejects_110m_scale() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(dir.path(), "scale: 110m\ntheme: HYP_HR_SR_OB_DR\n");
    let err = naturalearth_raster::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("scale"));
}
