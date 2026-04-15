mod common;

use std::fs;
use std::sync::Mutex;

use data::catalog::census_acs;

use crate::common::{work_dir, write_params};

static ENV_LOCK: Mutex<()> = Mutex::new(());

#[tokio::test]
async fn acs_happy_path_with_key() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    let body = r#"[["NAME","B01003_001E","state"],["California","39000000","06"]]"#;
    Mock::given(method("GET"))
        .and(path("/2023/acs/acs5"))
        .respond_with(ResponseTemplate::new(200).set_body_string(body))
        .mount(&server)
        .await;
    let server_uri = server.uri();

    let handle = tokio::task::spawn_blocking(move || {
        let _g = ENV_LOCK.lock().unwrap();
        let dir = work_dir();
        unsafe {
            std::env::set_var("CENSUS_ACS_BASE_URL", &server_uri);
            std::env::set_var("CENSUS_API_KEY", "dummy");
        }
        write_params(
            dir.path(),
            "year: 2023\ndataset: acs5\ntable: B01003\ngeography: 'state:*'\nvariables: ''\n",
        );
        let r = census_acs::run_in(dir.path());
        unsafe {
            std::env::remove_var("CENSUS_ACS_BASE_URL");
            std::env::remove_var("CENSUS_API_KEY");
        }
        (dir, r)
    });
    let (dir, result) = handle.await.unwrap();
    result.unwrap();

    let out = dir.path().join("outputs/tabular/acs.csv");
    assert!(out.exists());
    let body = fs::read_to_string(&out).unwrap();
    assert!(body.starts_with("NAME,B01003_001E,state"));
    assert!(body.contains("California"));
}

#[test]
fn acs_without_key_errors() {
    let _g = ENV_LOCK.lock().unwrap();
    // Ensure no stray key lingers.
    unsafe {
        std::env::remove_var("CENSUS_API_KEY");
    }
    let dir = work_dir();
    write_params(
        dir.path(),
        "year: 2023\ndataset: acs5\ntable: B01003\ngeography: 'state:*'\nvariables: ''\n",
    );
    let err = census_acs::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("CENSUS_API_KEY"));
}

#[test]
fn bad_dataset_rejected() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    unsafe {
        std::env::set_var("CENSUS_API_KEY", "x");
    }
    write_params(
        dir.path(),
        "year: 2023\ndataset: acsX\ntable: B01003\ngeography: 'state:*'\nvariables: ''\n",
    );
    let err = census_acs::run_in(dir.path()).unwrap_err();
    unsafe {
        std::env::remove_var("CENSUS_API_KEY");
    }
    assert!(err.to_string().contains("dataset"));
}
