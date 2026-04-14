mod common;

use std::fs;

use base::common::expansion::ExpansionManifest;
use base::map_list;

use crate::common::{work_dir, write_params};

#[test]
fn fan_out_over_strings() {
    let dir = work_dir();
    write_params(dir.path(), "values: '[\"NY\", \"MI\", \"CA\"]'\n");
    map_list::run_in(dir.path()).unwrap();

    let manifest_path = dir.path().join("outputs/manifest/expansion.yaml");
    let yaml = fs::read_to_string(&manifest_path).unwrap();
    let parsed: ExpansionManifest = serde_yaml::from_str(&yaml).unwrap();
    assert_eq!(parsed.items.len(), 3);
    let keys: Vec<&str> = parsed.items.iter().map(|i| i.key.as_str()).collect();
    assert_eq!(keys, vec!["NY", "MI", "CA"]);

    // Each item JSON file exists under outputs/manifest/items/.
    for item in &parsed.items {
        let full = dir.path().join(&item.path);
        assert!(full.exists(), "missing item file {}", item.path);
    }
}

#[test]
fn fan_out_over_numbers() {
    let dir = work_dir();
    write_params(dir.path(), "values: '[1, 2, 3]'\n");
    map_list::run_in(dir.path()).unwrap();

    let yaml = fs::read_to_string(dir.path().join("outputs/manifest/expansion.yaml")).unwrap();
    let parsed: ExpansionManifest = serde_yaml::from_str(&yaml).unwrap();
    let keys: Vec<&str> = parsed.items.iter().map(|i| i.key.as_str()).collect();
    assert_eq!(keys, vec!["1", "2", "3"]);
}

#[test]
fn empty_list_rejected() {
    let dir = work_dir();
    write_params(dir.path(), "values: '[]'\n");
    let err = map_list::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("empty"));
}

#[test]
fn malformed_json_rejected() {
    let dir = work_dir();
    write_params(dir.path(), "values: \"not json\"\n");
    let err = map_list::run_in(dir.path()).unwrap_err();
    let msg = err.to_string().to_lowercase();
    assert!(msg.contains("json") || msg.contains("expected"));
}

#[test]
fn nested_values_rejected() {
    let dir = work_dir();
    write_params(dir.path(), "values: '[[1,2],[3,4]]'\n");
    let err = map_list::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("scalar"));
}
