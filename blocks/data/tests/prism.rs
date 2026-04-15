mod common;

use std::fs;
use std::sync::Mutex;

use data::catalog::prism;
use data::common::download::build_test_zip;

use crate::common::{work_dir, write_params};

static ENV_LOCK: Mutex<()> = Mutex::new(());

#[tokio::test]
async fn prism_monthly_happy_path() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    let zip1 = build_test_zip(&[("ppt_202301.bil", b"BIL1"), ("ppt_202301.hdr", b"HDR1")]).unwrap();
    let zip2 = build_test_zip(&[("ppt_202302.bil", b"BIL2"), ("ppt_202302.hdr", b"HDR2")]).unwrap();
    Mock::given(method("GET"))
        .and(path("/ppt/monthly/202301"))
        .respond_with(ResponseTemplate::new(200).set_body_bytes(zip1))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/ppt/monthly/202302"))
        .respond_with(ResponseTemplate::new(200).set_body_bytes(zip2))
        .mount(&server)
        .await;
    let server_uri = server.uri();

    let handle = tokio::task::spawn_blocking(move || {
        let _g = ENV_LOCK.lock().unwrap();
        let dir = work_dir();
        unsafe {
            std::env::set_var("PRISM_BASE_URL", &server_uri);
        }
        write_params(
            dir.path(),
            "variable: ppt\nstart: '2023-01-01'\nend: '2023-02-01'\nresolution: 4km\ncadence: monthly\n",
        );
        let r = prism::run_in(dir.path());
        unsafe {
            std::env::remove_var("PRISM_BASE_URL");
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
fn rejects_800m() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(
        dir.path(),
        "variable: ppt\nstart: '2023-01-01'\nend: '2023-01-01'\nresolution: 800m\ncadence: monthly\n",
    );
    let err = prism::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("paywalled"));
}

#[test]
fn rejects_unknown_variable() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(
        dir.path(),
        "variable: bogus\nstart: '2023-01-01'\nend: '2023-01-01'\nresolution: 4km\ncadence: monthly\n",
    );
    let err = prism::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("variable"));
}
