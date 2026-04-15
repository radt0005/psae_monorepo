mod common;

use std::fs;

use data::read;

use crate::common::{put_input, work_dir, write_params};

#[test]
fn file_scheme_happy_path() {
    let dir = work_dir();
    let src = dir.path().join("hello.txt");
    fs::write(&src, b"hello world").unwrap();
    let uri = format!("file://{}", src.to_string_lossy());

    write_params(dir.path(), &format!("uri: {uri}\nformat: ''\n"));
    read::run_in(dir.path()).unwrap();

    // Output goes to outputs/file/<basename>.
    let out = dir.path().join("outputs/file/hello.txt");
    assert!(out.exists(), "expected output {:?}", out);
    assert_eq!(fs::read(&out).unwrap(), b"hello world");
}

#[test]
fn bare_absolute_path() {
    let dir = work_dir();
    let src = dir.path().join("hello.txt");
    fs::write(&src, b"abc").unwrap();

    write_params(
        dir.path(),
        &format!("uri: {}\nformat: ''\n", src.to_string_lossy()),
    );
    read::run_in(dir.path()).unwrap();

    let out = dir.path().join("outputs/file/hello.txt");
    assert!(out.exists());
    assert_eq!(fs::read(&out).unwrap(), b"abc");
}

#[test]
fn empty_uri_is_bad_argument() {
    let dir = work_dir();
    write_params(dir.path(), "uri: ''\nformat: ''\n");
    let err = read::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("uri"));
}

#[test]
fn unsupported_scheme_errs() {
    let dir = work_dir();
    write_params(dir.path(), "uri: 'ipfs://QmFoo/bar'\nformat: ''\n");
    let err = read::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().to_lowercase().contains("unsupported"));
}

#[test]
fn format_sidecar_written() {
    let dir = work_dir();
    let src = dir.path().join("x.csv");
    fs::write(&src, b"a,b\n1,2\n").unwrap();
    let uri = format!("file://{}", src.to_string_lossy());

    write_params(dir.path(), &format!("uri: {uri}\nformat: CSV\n"));
    read::run_in(dir.path()).unwrap();

    let sidecar = dir.path().join("invocation.json");
    assert!(sidecar.exists());
    let body = fs::read_to_string(&sidecar).unwrap();
    assert!(body.contains("\"format\""));
    assert!(body.contains("CSV"));
}

// ---------------------------------------------------------------------------
// HTTP tests via wiremock
// ---------------------------------------------------------------------------

#[tokio::test]
async fn https_happy_path() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/data.bin"))
        .respond_with(ResponseTemplate::new(200).set_body_bytes(b"wiremock bytes" as &[u8]))
        .mount(&server)
        .await;

    // Run the handler on a blocking thread since our runtime::block_on
    // uses a thread-local current-thread tokio runtime.
    let server_uri = server.uri();
    let handle = tokio::task::spawn_blocking(move || {
        let dir = work_dir();
        let uri = format!("{server_uri}/data.bin");
        write_params(dir.path(), &format!("uri: {uri}\nformat: ''\n"));
        let r = read::run_in(dir.path());
        (dir, r)
    });
    let (dir, result) = handle.await.unwrap();
    result.unwrap();

    let out = dir.path().join("outputs/file/data.bin");
    assert!(out.exists());
    assert_eq!(fs::read(&out).unwrap(), b"wiremock bytes");
}

#[tokio::test]
async fn https_404_not_found() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/missing.bin"))
        .respond_with(ResponseTemplate::new(404))
        .mount(&server)
        .await;

    let server_uri = server.uri();
    let handle = tokio::task::spawn_blocking(move || {
        let dir = work_dir();
        let uri = format!("{server_uri}/missing.bin");
        write_params(dir.path(), &format!("uri: {uri}\nformat: ''\n"));
        let r = read::run_in(dir.path());
        (dir, r)
    });
    let (_dir, result) = handle.await.unwrap();
    let err = result.unwrap_err();
    let msg = err.to_string().to_lowercase();
    assert!(msg.contains("not found") || msg.contains("notfound"));
}

#[tokio::test]
async fn https_403_unauthorized() {
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/secret.bin"))
        .respond_with(ResponseTemplate::new(403))
        .mount(&server)
        .await;

    let server_uri = server.uri();
    let handle = tokio::task::spawn_blocking(move || {
        let dir = work_dir();
        let uri = format!("{server_uri}/secret.bin");
        write_params(dir.path(), &format!("uri: {uri}\nformat: ''\n"));
        let r = read::run_in(dir.path());
        (dir, r)
    });
    let (_dir, result) = handle.await.unwrap();
    let err = result.unwrap_err();
    let msg = err.to_string().to_lowercase();
    // PermissionDenied or 403/unauthorized
    assert!(
        msg.contains("unauthorized") || msg.contains("permission") || msg.contains("403"),
        "unexpected error: {msg}"
    );
    let _ = put_input; // silence warning when this fn is unused elsewhere
}
