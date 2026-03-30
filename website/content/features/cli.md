+++
title = "CLI + Web UI"
weight = 6
description = "Develop locally with the CLI. Deploy and collaborate through the web interface."
template = "features/page.html"
+++

Spade provides both a powerful command-line interface for developers and a visual web interface for teams.

## CLI Commands

The Spade CLI is your primary development tool:

- **`spade init <language>`** — Scaffold a new block collection in your preferred language
- **`spade add <name>`** — Add a new block to your collection with manifest and entry point boilerplate
- **`spade check`** — Validate pipeline definitions and block manifests
- **`spade run <pipeline.yaml>`** — Execute a pipeline locally for testing
- **`spade install <git-url>`** — Install a block collection from a Git repository
- **`spade upload`** — Package and upload your collection for cloud deployment
- **`spade setup`** — Configure the Spade system and dependencies

## Web Interface

The web UI provides a visual flowchart editor for building pipelines:

- **Drag-and-drop** block placement on a canvas
- **Smart wiring** — Automatic connection resolution when types are unambiguous
- **Pipeline management** — Save, share, and re-run pipelines
- **Result visualization** — View outputs and download result files
- **Block browsing** — Explore available blocks with metadata and documentation
- **Collaboration** — Share pipelines with team members

## Local to Cloud

Develop and test locally with `spade run`, then deploy the same pipeline to a multi-worker cloud environment — no changes required. The scheduler handles distribution automatically.
