mod common;

use std::fs;
use std::sync::Mutex;

use data::catalog::fia;
use data::common::download::build_test_zip;

use crate::common::{work_dir, write_params};

// Tests in this file all mutate the FIA_BASE_URL env var, which is
// process-global. Serialise them.
static ENV_LOCK: Mutex<()> = Mutex::new(());

fn fixture_zip() -> Vec<u8> {
    build_test_zip(&[
        ("NY_PLOT.csv", b"CN,INVYR\n1,2020\n"),
        ("NY_TREE.csv", b"CN,DIA\n1,10\n"),
        ("NY_COND.csv", b"CN,CONDID\n1,1\n"),
    ])
    .unwrap()
}

async fn with_mock_fia<F, T>(
    path_segment: &str,
    response: wiremock::ResponseTemplate,
    params_yaml: String,
    body: F,
) -> T
where
    F: FnOnce(tempfile::TempDir, data::common::Result<()>) -> T + Send + 'static,
    T: Send + 'static,
{
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer};

    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path(path_segment.to_string()))
        .respond_with(response)
        .mount(&server)
        .await;
    let server_uri = server.uri();

    tokio::task::spawn_blocking(move || {
        let _g = ENV_LOCK.lock().unwrap();
        let dir = work_dir();
        // SAFETY: serialised by ENV_LOCK.
        unsafe {
            std::env::set_var("FIA_BASE_URL", &server_uri);
        }
        write_params(dir.path(), &params_yaml);
        let r = fia::run_in(dir.path());
        unsafe {
            std::env::remove_var("FIA_BASE_URL");
        }
        body(dir, r)
    })
    .await
    .unwrap()
}

#[tokio::test]
async fn fia_state_with_tables_filter() {
    use wiremock::ResponseTemplate;
    with_mock_fia(
        "/NY_CSV.zip",
        ResponseTemplate::new(200).set_body_bytes(fixture_zip()),
        "state: NY\ntables: PLOT,TREE\n".to_string(),
        |dir, result| {
            result.unwrap();
            let out_dir = dir.path().join("outputs/tables");
            let mut names: Vec<String> = fs::read_dir(&out_dir)
                .unwrap()
                .map(|e| e.unwrap().file_name().to_string_lossy().to_string())
                .collect();
            names.sort();
            assert_eq!(names, vec!["NY_PLOT.csv".to_string(), "NY_TREE.csv".into()]);
        },
    )
    .await;
}

#[tokio::test]
async fn fia_all_tables_extracted_when_empty_filter() {
    use wiremock::ResponseTemplate;
    with_mock_fia(
        "/NY_CSV.zip",
        ResponseTemplate::new(200).set_body_bytes(fixture_zip()),
        "state: NY\ntables: ''\n".to_string(),
        |dir, result| {
            result.unwrap();
            let out_dir = dir.path().join("outputs/tables");
            let count = fs::read_dir(&out_dir).unwrap().count();
            assert_eq!(count, 3);
        },
    )
    .await;
}

#[test]
fn fia_bad_state_rejected() {
    let _g = ENV_LOCK.lock().unwrap();
    let dir = work_dir();
    write_params(dir.path(), "state: ZZ\ntables: ''\n");
    let err = fia::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().to_lowercase().contains("unknown state"));
}

#[tokio::test]
async fn fia_404_is_not_found() {
    use wiremock::ResponseTemplate;
    with_mock_fia(
        "/NY_CSV.zip",
        ResponseTemplate::new(404),
        "state: NY\ntables: ''\n".to_string(),
        |_dir, result| {
            let err = result.unwrap_err();
            assert!(err.to_string().to_lowercase().contains("not found"));
        },
    )
    .await;
}

#[tokio::test]
async fn fia_nonmatching_tables_errors() {
    use wiremock::ResponseTemplate;
    with_mock_fia(
        "/NY_CSV.zip",
        ResponseTemplate::new(200).set_body_bytes(fixture_zip()),
        "state: NY\ntables: NONEXISTENT\n".to_string(),
        |_dir, result| {
            let err = result.unwrap_err();
            assert!(err.to_string().contains("no tables matched"));
        },
    )
    .await;
}

#[tokio::test]
async fn fia_malformed_zip_errors() {
    use wiremock::ResponseTemplate;
    with_mock_fia(
        "/NY_CSV.zip",
        ResponseTemplate::new(200).set_body_bytes(b"not a zip" as &[u8]),
        "state: NY\ntables: ''\n".to_string(),
        |_dir, result| {
            let err = result.unwrap_err();
            let msg = err.to_string().to_lowercase();
            assert!(msg.contains("zip") || msg.contains("invalid"), "got: {msg}");
        },
    )
    .await;
}
