+++
title = "Python"
description = "Build Spade blocks in Python using familiar tools and type hints."
weight = 1
sort_by = "weight"
insert_anchor_links = "right"
+++

The Python library lets you write Spade blocks as plain Python functions with type hints. The library handles reading inputs, loading parameters from `params.yaml`, and writing outputs.

## Prerequisites

- **Python 3.12 or later**
- **`uv`** package manager (recommended) or `pip`
- The Spade CLI installed ([Installation guide](/getting-started/installation/))

## Installation

Add `spade` as a dependency in your `pyproject.toml`, or install it directly:

```bash
pip install spade
```

With `uv`:

```bash
uv add spade
```
