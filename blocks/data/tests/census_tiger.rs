mod common;

use std::fs;
use std::sync::Mutex;

use data::catalog::census_tiger;
use data::common::download::build_test_zip;

use crate::common::{work_dir, write_params};

static ENV_LOCK: Mutex<()> = Mutex::new(());

fn shapefile_zip() -> Vec<u8> {
    build_test_zip(&[
        ("tl_2023_us_state.shp", b"SHP"),
        ("tl_2023_us_state.dbf", b"DBF"),
        ("tl_2023_us_state.shx", b"SHX"),
        ("tl_2023_us_state.prj", b"PRJ"),
    ])
    .unwrap()
}

#[tokio::test]
async fn national_layer_happy_path() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/TIGER2023/STATE/tl_2023_us_state.zip"))
        .respond_with(ResponseTemplate::new(200).set_body_bytes(shapefile_zip()))
        .mount(&server)
        .await;
    let server_uri = server.uri();

    let handle = tokio::task::spawn_blocking(move || {
        let _g = ENV_LOCK.lock().unwrap();
        let dir = work_dir();
        unsafe {
            std::env::set_var("CENSUS_TIGER_BASE_URL", &server_uri);
        }
        write_params(dir.path(), "year: 2023\nlayer: states\nstate: ''\n");
        let r = census_tiger::run_in(dir.path());
        unsafe {
            std::env::remove_var("CENSUS_TIGER_BASE_URL");
        }
        (dir, r)
    });
    let (dir, result) = handle.await.unwrap();
    result.unwrap();

    // Directory output lands under outputs/directory/ due to the type's
    // default_output_name.
    let out = dir.path().join("outputs/directory");
    assert!(out.exists());
    let count = fs::read_dir(&out).unwrap().count();
    assert_eq!(count, 4);
}

#[test]
fn state_layer_without_state_fails() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(dir.path(), "year: 2023\nlayer: tracts\nstate: ''\n");
    let err = census_tiger::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("requires a state"));
}

#[test]
fn bad_year_rejected() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(dir.path(), "year: 1999\nlayer: states\nstate: ''\n");
    let err = census_tiger::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("year"));
}

#[test]
fn unknown_layer_rejected() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(dir.path(), "year: 2023\nlayer: nonsense\nstate: ''\n");
    let err = census_tiger::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("unknown layer"));
}
