mod common;

use std::fs;

use base::common::expansion::ExpansionManifest;
use base::map_range;

use crate::common::{work_dir, write_params};

#[test]
fn integer_range() {
    let dir = work_dir();
    write_params(dir.path(), "start: 0\nend: 5\nstep: 1\n");
    map_range::run_in(dir.path()).unwrap();
    let yaml = fs::read_to_string(dir.path().join("outputs/manifest/expansion.yaml")).unwrap();
    let parsed: ExpansionManifest = serde_yaml::from_str(&yaml).unwrap();
    assert_eq!(parsed.items.len(), 5);
}

#[test]
fn fractional_step() {
    let dir = work_dir();
    write_params(dir.path(), "start: 0.0\nend: 1.0\nstep: 0.25\n");
    map_range::run_in(dir.path()).unwrap();
    let yaml = fs::read_to_string(dir.path().join("outputs/manifest/expansion.yaml")).unwrap();
    let parsed: ExpansionManifest = serde_yaml::from_str(&yaml).unwrap();
    assert_eq!(parsed.items.len(), 4);
}

#[test]
fn zero_step_rejected() {
    let dir = work_dir();
    write_params(dir.path(), "start: 0\nend: 5\nstep: 0\n");
    let err = map_range::run_in(dir.path()).unwrap_err();
    assert!(err.to_string().contains("non-zero"));
}
