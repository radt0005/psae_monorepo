mod common;

use std::fs;

use base::common::expansion::ExpansionManifest;
use base::map_files;

use crate::common::{put_input, work_dir};

#[test]
fn enumerates_files_sorted() {
    let dir = work_dir();
    // Create them out of alphabetical order to exercise sorting.
    put_input(dir.path(), "source", "c.tif", b"c");
    put_input(dir.path(), "source", "a.tif", b"a");
    put_input(dir.path(), "source", "b.tif", b"b");

    map_files::run_in(dir.path()).unwrap();

    let manifest_path = dir.path().join("outputs/manifest/expansion.yaml");
    assert!(manifest_path.exists());
    let yaml = fs::read_to_string(&manifest_path).unwrap();
    let parsed: ExpansionManifest = serde_yaml::from_str(&yaml).unwrap();
    assert_eq!(parsed.items.len(), 3);
    let keys: Vec<&str> = parsed.items.iter().map(|i| i.key.as_str()).collect();
    assert_eq!(keys, vec!["a", "b", "c"]);
    // Paths are relative to the base dir.
    for item in &parsed.items {
        assert!(item.path.starts_with("inputs/source/"));
    }
}
