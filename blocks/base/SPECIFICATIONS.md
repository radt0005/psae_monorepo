# Core Block Specifications

This document contains specifications for the core set of Blocks for the spade system.  For more information on the core system, please see `../../spec/`.  This is intended to provide the commonly used data processing steps for the system.  

This system should be written in Rust for speed. I have initialized the repository.

## Blocks

The blocks we would like to create center around a few common use cases: 
1. Tabular data operations
2. Format conversion
3. Utilities


## Tabular Data

These should have common operations for working with tabular data.  Internally, this should output parquet files where possible, but should also support CSV inputs. 

These blocks are as follows: 
1. Filter operations on rows (like a SQL WHERE clause)
2. Filter columns (select only certain columns and drop the rest)
3. Aggregations (mean, median, mode, percentiles, )