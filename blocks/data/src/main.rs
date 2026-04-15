//! `data` binary dispatcher.
//!
//! The collection compiles to a single binary; each block is a
//! subcommand dispatched from `args[1]`.

use data::catalog;
use data::{list, read, read_collection, stat, write, write_collection};

fn main() {
    let mut args = std::env::args();
    let _exe = args.next();
    let block = match args.next() {
        Some(name) => name,
        None => {
            eprintln!("{}", usage());
            std::process::exit(2);
        }
    };

    match block.as_str() {
        // Generic
        "read" => read::entry(),
        "read_collection" => read_collection::entry(),
        "write" => write::entry(),
        "write_collection" => write_collection::entry(),
        "list" => list::entry(),
        "stat" => stat::entry(),

        // Catalog
        "fia" => catalog::fia::entry(),
        "census_tiger" => catalog::census_tiger::entry(),
        "census_acs" => catalog::census_acs::entry(),
        "naturalearth_vector" => catalog::naturalearth_vector::entry(),
        "naturalearth_raster" => catalog::naturalearth_raster::entry(),
        "usgs_3dep" => catalog::usgs_3dep::entry(),
        "nlcd" => catalog::nlcd::entry(),
        "nhd" => catalog::nhd::entry(),
        "ssurgo" => catalog::ssurgo::entry(),
        "prism" => catalog::prism::entry(),
        "osm_extract_pbf" => catalog::osm_extract_pbf::entry(),
        "osm_extract_shp" => catalog::osm_extract_shp::entry(),

        other => {
            eprintln!("unknown block: {other}\n\n{}", usage());
            std::process::exit(2);
        }
    }
}

fn usage() -> String {
    [
        "usage: data <block>",
        "",
        "generic blocks:",
        "  read",
        "  read_collection",
        "  write",
        "  write_collection",
        "  list",
        "  stat",
        "",
        "catalog blocks:",
        "  fia",
        "  census_tiger",
        "  census_acs",
        "  naturalearth_vector",
        "  naturalearth_raster",
        "  usgs_3dep",
        "  nlcd",
        "  nhd",
        "  ssurgo",
        "  prism",
        "  osm_extract_pbf",
        "  osm_extract_shp",
    ]
    .join("\n")
}
