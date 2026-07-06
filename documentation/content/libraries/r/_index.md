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
- **`pak`** for dependency management (recommended)
- The Spade CLI installed ([Installation guide](/getting-started/installation/))

## Dependencies

A collection declares its CRAN dependencies in a `DESCRIPTION` file under
`Imports:`. `pak` resolves them into a `pkg.lock` at build time and installs them
into the collection's library.

```
Imports:
    jsonlite,
    yaml
```

## Installation

```r
pak::pkg_install("spade")
```
