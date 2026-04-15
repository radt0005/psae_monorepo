mod common;

use std::sync::Mutex;

use data::catalog::ssurgo;
use data::common::download::build_test_zip;

use crate::common::{work_dir, write_params};

static ENV_LOCK: Mutex<()> = Mutex::new(());

#[tokio::test]
async fn ssurgo_area_happy_path() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    let zip = build_test_zip(&[("soildb.gdb", b"GDB")]).unwrap();
    Mock::given(method("GET"))
        .and(path("/wss_SSA_CA077_soildb_US_2003.zip"))
        .respond_with(ResponseTemplate::new(200).set_body_bytes(zip))
        .mount(&server)
        .await;
    let server_uri = server.uri();

    let handle = tokio::task::spawn_blocking(move || {
        let _g = ENV_LOCK.lock().unwrap();
        let dir = work_dir();
        unsafe {
            std::env::set_var("SSURGO_BASE_URL", &server_uri);
        }
        write_params(dir.path(), "area: CA077\nstate: ''\n");
        let r = ssurgo::run_in(dir.path());
        unsafe {
            std::env::remove_var("SSURGO_BASE_URL");
        }
        (dir, r)
    });
    let (dir, result) = handle.await.unwrap();
    result.unwrap();

    let out = dir.path().join("outputs/directory/soildb.gdb");
    assert!(out.exists());
}

#[test]
fn requires_exactly_one_selector() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(dir.path(), "area: ''\nstate: ''\n");
    let err = ssurgo::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("exactly one"));

    let dir = work_dir();
    write_params(dir.path(), "area: CA077\nstate: CA\n");
    let err = ssurgo::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("exactly one"));
}
