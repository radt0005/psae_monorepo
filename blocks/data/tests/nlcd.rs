mod common;

use std::fs;
use std::sync::Mutex;

use data::catalog::nlcd;
use data::common::download::build_test_zip;

use crate::common::{work_dir, write_params};

static ENV_LOCK: Mutex<()> = Mutex::new(());

#[tokio::test]
async fn nlcd_happy_path() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    let zip = build_test_zip(&[("nlcd_2021_land_cover_CONUS.tif", b"TIFF")]).unwrap();
    Mock::given(method("GET"))
        .and(path("/nlcd_2021_land_cover_CONUS.zip"))
        .respond_with(ResponseTemplate::new(200).set_body_bytes(zip))
        .mount(&server)
        .await;
    let server_uri = server.uri();

    let handle = tokio::task::spawn_blocking(move || {
        let _g = ENV_LOCK.lock().unwrap();
        let dir = work_dir();
        unsafe {
            std::env::set_var("NLCD_BASE_URL", &server_uri);
        }
        write_params(
            dir.path(),
            "year: 2021\nproduct: land_cover\nregion: CONUS\n",
        );
        let r = nlcd::run_in(dir.path());
        unsafe {
            std::env::remove_var("NLCD_BASE_URL");
        }
        (dir, r)
    });
    let (dir, result) = handle.await.unwrap();
    result.unwrap();

    let out = dir.path().join("outputs/raster/nlcd_2021_land_cover_CONUS.tif");
    assert!(out.exists(), "expected {:?}", out);
    assert_eq!(fs::read(&out).unwrap(), b"TIFF");
}

#[test]
fn bad_year_rejected() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(
        dir.path(),
        "year: 2022\nproduct: land_cover\nregion: CONUS\n",
    );
    let err = nlcd::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("year"));
}

#[test]
fn bad_product_rejected() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(dir.path(), "year: 2021\nproduct: bogus\nregion: CONUS\n");
    let err = nlcd::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("product"));
}
