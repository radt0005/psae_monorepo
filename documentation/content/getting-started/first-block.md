+++
title = "Your First Block"
description = "Create a custom processing block from scratch using Python."
weight = 3
+++

In this guide, you'll create a custom block that reads a CSV file, computes basic summary statistics, and writes the results as JSON. We'll use Python, but the same workflow applies to any supported language.

## Step 1: Create a block collection

A **collection** is a repository of related blocks that share a language. Create one:

```bash
mkdir csv-stats && cd csv-stats
spade init --language python
```

This scaffolds a Python project:

```
csv-stats/
  pyproject.toml    # Python package configuration
  src/
    csv_stats/
      __init__.py
  blocks/           # Block manifests go here
```

## Step 2: Add a block

```bash
spade add summarize
```

This creates two files:

1. **`blocks/summarize.yaml`** — The block manifest declaring inputs, outputs, and parameters
2. **`src/csv_stats/summarize.py`** — The handler function you'll implement

## Step 3: Define the block manifest

Edit `blocks/summarize.yaml` to declare what your block accepts and produces:

```yaml
id: csv-stats.summarize
version: 0.1.0
kind: standard
network: false
description: Computes summary statistics for a CSV file

inputs:
  data:
    type: file
    format: CSV
    description: The input CSV file to analyze
  column:
    type: string
    description: Name of the column to summarize

outputs:
  stats:
    type: json
    description: JSON file containing computed statistics
```

This manifest tells Spade that the block:

- Accepts a CSV file input named `data`
- Accepts a string parameter named `column`
- Produces a JSON file output named `stats`
- Does not need network access
- Is a standard (non-map, non-reduce) block

## Step 4: Implement the handler

Edit `src/csv_stats/summarize.py`:

```python
import csv
import json
from spade import run, TabularFile, JsonFile


def handler(data: TabularFile, column: str) -> JsonFile:
    """Compute summary statistics for a column in a CSV file."""
    values = []
    with open(data.path) as f:
        reader = csv.DictReader(f)
        for row in reader:
            try:
                values.append(float(row[column]))
            except (ValueError, KeyError):
                continue

    if not values:
        stats = {"error": f"No numeric values found in column '{column}'"}
    else:
        stats = {
            "column": column,
            "count": len(values),
            "mean": sum(values) / len(values),
            "min": min(values),
            "max": max(values),
        }

    output_path = "outputs/stats/summary.json"
    with open(output_path, "w") as f:
        json.dump(stats, f, indent=2)

    return JsonFile(path=output_path)


if __name__ == "__main__":
    run(handler)
```

Key points:

- The **function signature** declares the types. `data: TabularFile` tells the Spade library to look for the input named `data` and load it as a `TabularFile` (which provides a `.path` attribute). `column: str` comes from `params.yaml` as a scalar parameter.
- The **return type** `JsonFile` tells Spade what kind of output to write.
- The function writes results to `outputs/stats/` — the output directory matching the manifest's output name.
- The `run(handler)` call at the bottom is the entry point. It handles loading inputs and parameters automatically.

## Step 5: Validate

Check that the manifest and source file are valid:

```bash
spade check
```

Expected output:

```
Collection 'csv-stats' (python) is valid.
  1 block found: csv-stats.summarize
```

## Step 6: Install locally

Install the collection from the local directory:

```bash
spade install file://.
```

This builds the Python package and registers the block in `~/.spade/blocks/csv-stats/0.1.0/`.

## Step 7: Use in a pipeline

Now create a pipeline that uses your new block. Create `test-pipeline.yaml`:

```yaml
id: 019cf4bc-a000-7000-0000-000000000000
name: csv-stats-test
version: "1.0"
description: Test the summarize block

blocks:
  - id: 019cf4bc-a001-7000-0000-000000000000
    name: csv-stats.summarize
    inputs: []
    args:
      column: temperature
```

{% note() %}
For this test, you'll need to provide an input CSV file. In a real pipeline, the input would come from an upstream block. For local testing, you can create a test working directory manually — see the [Testing Blocks](/tutorials/testing-blocks/) tutorial.
{% end %}

## Next steps

- Learn about [block types and the manifest format](/concepts/blocks/) in detail
- Explore [library documentation](/libraries/) for your preferred language
- Read the [Building a Block](/tutorials/building-a-block/) tutorial for a comprehensive walkthrough
