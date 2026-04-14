//! Base block collection — core data-processing blocks for Spade.

pub mod common;

pub mod aggregate;
pub mod csv_to_parquet;
pub mod filter_rows;
pub mod group_by;
pub mod map_files;
pub mod map_list;
pub mod map_range;
pub mod parquet_to_csv;
pub mod reduce_collection;
pub mod reduce_join;
pub mod reduce_stack;
pub mod select_columns;
