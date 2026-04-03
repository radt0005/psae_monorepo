+++
title = "Spade Documentation"
template = "index.html"
+++

# Welcome to Spade

Spade is an open-source system for building **reproducible data processing workflows**, with first-class support for geospatial data. You define processing steps as independent, reusable **blocks**, connect them into **pipelines** using simple YAML files, and let Spade handle execution, caching, and parallelism.

Blocks can be written in **Python**, **R**, **TypeScript**, **Go**, or **Rust** — use whichever language best fits your domain. Spade runs each block in an isolated sandbox, so you can mix languages freely within a single pipeline.

### What you'll find here

- **[Getting Started](/getting-started/)** — Install Spade, run your first pipeline, and create your first block
- **[Core Concepts](/concepts/)** — Understand blocks, pipelines, map/reduce, and how Spade executes workflows
- **[Pipeline Reference](/pipelines/)** — Complete reference for writing YAML pipeline files
- **[CLI Reference](/cli/)** — Documentation for every `spade` command
- **[Block Development Libraries](/libraries/)** — Language-specific guides for Python, R, TypeScript, Go, and Rust
- **[Tutorials](/tutorials/)** — End-to-end walkthroughs for common tasks
