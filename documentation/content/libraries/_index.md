+++
title = "Block Development Libraries"
description = "Python, R, TypeScript, Go, and Rust libraries for building blocks."
weight = 5
sort_by = "weight"
insert_anchor_links = "right"
+++

Spade provides official libraries for five languages. Each library handles the runtime details — reading inputs, loading parameters, writing outputs — so you can focus entirely on your processing logic.

| Language | Runtime | Install |
|----------|---------|---------|
| [Python](/libraries/python/) | Python 3.12+ with `uv` | `pip install spade` |
| [R](/libraries/r/) | R 4.0+ with `renv` | `install.packages("spade")` |
| [TypeScript](/libraries/typescript/) | Bun 1.0+ | `bun add spade` |
| [Go](/libraries/go/) | Go 1.25+ | `go get github.com/spade-dev/spade` |
| [Rust](/libraries/rust/) | Rust stable | `cargo add spade` |

All five libraries share the same concepts and workflow:

1. **Define a handler function** that accepts typed inputs and parameters
2. **Process the data** using your domain tools and libraries
3. **Return a typed output** — the library writes it to the correct location
4. **Call `run(handler)`** — the library wires everything together

The libraries also provide a **manifest builder** that generates block manifest YAML from your handler's type annotations, keeping the manifest and implementation in sync.
