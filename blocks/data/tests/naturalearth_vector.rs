mod common;

use std::fs;
use std::sync::Mutex;

use data::catalog::naturalearth_vector;
use data::common::download::build_test_zip;

use crate::common::{work_dir, write_params};

static ENV_LOCK: Mutex<()> = Mutex::new(());

#[tokio::test]
async fn happy_path_cultural_10m() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    let zip = build_test_zip(&[
        ("ne_10m_admin_0_countries.shp", b"SHP"),
        ("ne_10m_admin_0_countries.dbf", b"DBF"),
    ])
    .unwrap();
    Mock::given(method("GET"))
        .and(path("/10m/cultural/ne_10m_admin_0_countries.zip"))
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
        write_params(
            dir.path(),
            "scale: 10m\ncategory: cultural\ntheme: admin_0_countries\n",
        );
        let r = naturalearth_vector::run_in(dir.path());
        unsafe {
            std::env::remove_var("NATURALEARTH_BASE_URL");
        }
        (dir, r)
    });
    let (dir, result) = handle.await.unwrap();
    result.unwrap();

    let out = dir.path().join("outputs/directory");
    assert!(out.exists());
    let count = fs::read_dir(&out).unwrap().count();
    assert_eq!(count, 2);
}

#[test]
fn unknown_scale_rejected() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(
        dir.path(),
        "scale: 42m\ncategory: cultural\ntheme: admin_0_countries\n",
    );
    let err = naturalearth_vector::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("scale"));
}
