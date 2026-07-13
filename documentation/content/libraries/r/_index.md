+++
title = "R"
description = "Build Spade blocks in R using familiar R idioms and packages."
weight = 2
sort_by = "weight"
insert_anchor_links = "right"
+++

The R library lets you write Spade blocks using familiar R patterns — reading YAML, processing data with base R or tidyverse packages, and writing results. Type annotations use R's S4 class system via simple attributes.

## Prerequisites

- **R 4.0 or later**
- **`renv`** for dependency locking, with **`pak`** as its recommended installer backend
- The Spade CLI installed ([Installation guide](/getting-started/installation/))

## Dependencies

A collection declares its CRAN dependencies in a `DESCRIPTION` file under `Imports:`:

```
Imports:
    jsonlite,
    yaml
```

Running `renv::snapshot()` resolves those declared dependencies (using `pak` under the hood for fast, parallel installs) and records the exact versions into `renv.lock` at the collection root:

```r
install.packages("jsonlite")
renv::snapshot()
```

`renv.lock` is the file `spade install` and the registry's build step actually read to reproduce the collection's library -- `DESCRIPTION` is where you declare what you depend on, `renv.lock` is the resolved, reproducible record of it.

## Installation

```r
install.packages("spade")
renv::snapshot()
```
