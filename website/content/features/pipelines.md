+++
title = "Declarative Pipelines"
weight = 2
description = "Define workflows as YAML DAGs. The scheduler handles the rest."
template = "features/page.html"
+++

Spade pipelines are declarative YAML files that describe a directed acyclic graph (DAG) of processing blocks and their dependencies.

## Pipeline Format

A pipeline specifies the blocks to execute and how their inputs and outputs connect:

```yaml
id: "01912345-6789-7abc-def0-123456789abc"
name: satellite-analysis
blocks:
  - id: "01912345-0001-7abc-def0-123456789abc"
    name: fetch-imagery
    args:
      region: "us-west"
      date_range: "2024-01-01/2024-03-01"
  - id: "01912345-0002-7abc-def0-123456789abc"
    name: reproject
    inputs:
      - "01912345-0001-7abc-def0-123456789abc"
  - id: "01912345-0003-7abc-def0-123456789abc"
    name: analyze
    inputs:
      - "01912345-0002-7abc-def0-123456789abc"
```

## Input Wiring

Inputs can be specified in two ways:

- **Simple references** — Just the block ID. Spade uses type matching to resolve which output connects to which input.
- **Explicit references** — Specify both the block and output name for full control: `{ block: "<id>", output: "result" }`

## Validation

Run `spade check` to validate your pipeline before execution. It verifies:

- All referenced blocks are installed
- Input/output types are compatible
- The dependency graph is a valid DAG (no cycles)
- Required parameters are provided
