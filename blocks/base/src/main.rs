use base::{
    aggregate, csv_to_parquet, filter_rows, group_by, join, map_files, map_list, map_range, mutate,
    parquet_to_csv, pivot, reduce_collection, reduce_join, reduce_stack, select_columns,
};

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
        "aggregate" => aggregate::entry(),
        "csv_to_parquet" => csv_to_parquet::entry(),
        "filter_rows" => filter_rows::entry(),
        "group_by" => group_by::entry(),
        "join" => join::entry(),
        "map_files" => map_files::entry(),
        "map_list" => map_list::entry(),
        "map_range" => map_range::entry(),
        "mutate" => mutate::entry(),
        "parquet_to_csv" => parquet_to_csv::entry(),
        "pivot" => pivot::entry(),
        "reduce_collection" => reduce_collection::entry(),
        "reduce_join" => reduce_join::entry(),
        "reduce_stack" => reduce_stack::entry(),
        "select_columns" => select_columns::entry(),
        other => {
            eprintln!("unknown block: {other}\n\n{}", usage());
            std::process::exit(2);
        }
    }
}

fn usage() -> String {
    [
        "usage: base <block>",
        "",
        "blocks:",
        "  aggregate",
        "  csv_to_parquet",
        "  filter_rows",
        "  group_by",
        "  join",
        "  map_files",
        "  map_list",
        "  map_range",
        "  mutate",
        "  parquet_to_csv",
        "  pivot",
        "  reduce_collection",
        "  reduce_join",
        "  reduce_stack",
        "  select_columns",
    ]
    .join("\n")
}
