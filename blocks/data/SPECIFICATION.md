# Specification

Please read the skill at `../../skills/spade` and the overall system specification in `../../spec` if you have not already.  These files have the explanation of the overall system.

This collection of blocks (plugins) will enable importing data into the spade system.  This means that most of the data that gets into the system will come through these blocks.  Based on our research, the plan is to use the Apache Open Data Access Layer (OpenDAL) for data import across formats.  This means that the data formats supported by OpenDAL can easily be supported by this system.  For this reason, we plan to use Rust for this collection of blocks.  The documentation and list of supported formats starts here: [https://opendal.apache.org/core/]

