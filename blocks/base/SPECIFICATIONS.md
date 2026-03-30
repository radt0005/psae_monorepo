# Core Block Specifications

This document contains specifications for the core set of Blocks for the spade system.  For more information on the core system, please see `../../spec/`.  This is intended to provide the commonly used data processing steps for the system.  

This system should be written in Rust for speed. I have initialized the repository.

## Blocks

The blocks we would like to create center around a few common use cases: 
1. Tabular data operations
2. Map
3. Reduce
4. A few utility blocks may be needed for things like file format conversion (e.g. CSV to Parquet)

## Tabular Data

These should have common operations for working with tabular data.  Internally, this should output parquet files where possible, but should also support CSV inputs. 

These blocks are as follows: 
1. Filter operations on rows (like a SQL WHERE clause)
2. Filter columns (select only certain columns and drop the rest)
3. Aggregations (mean, median, mode, percentiles, counts, )
4. grouping operations (Group By)

## Map operations

There should be blocks for the following types of map operations
1. Apply one or more blocks to each member of a file collection

## Reduce Operations

There should be a few types of reducers based on how the outputs are to be combined
1. Return the outputs back to being a FileCollection (this is trivial, just moves outputs back together); it should be the minimum viable reducer.
2. Stack tables (concatenate dataframes, like R's `rbind`)
3. SQL JOIN the outputs (not using actual SQL necessarily, just using that to express the idea of joining the tables in that fashion)



## Hints

See `../../skill.md` for a skill using the CLI to make blocks.