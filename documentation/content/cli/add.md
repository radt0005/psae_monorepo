+++
title = "spade add"
description = "Add a new block to the current collection."
weight = 3
+++

The `spade add` command creates a block manifest and a language-appropriate source file for a new block in the current collection directory.

## Usage

```bash
spade add <name>
```

The command must be run from the root of a block collection (a directory containing a language marker file such as `Cargo.toml`, `go.mod`, `pyproject.toml`, `package.json`, or `renv.lock`). The language is auto-detected from the marker file.

The `<name>` argument is the block name. It is used to form the block ID as `<collection>.<name>`, where `<collection>` is the current directory name.

## What it creates

Two files are created:

1. **`blocks/<name>.yaml`** -- The block manifest declaring the block's identity, inputs, and outputs.
2. **A source file** -- A language-appropriate handler file with stub code.

### Default manifest template

The generated manifest has this structure:

```yaml
id: my-collection.my-block
version: 0.1.0
kind: standard
network: false
description: ""
entrypoint: my-block
inputs: {}
outputs: {}
```

Fields:

| Field | Default | Description |
|-------|---------|-------------|
| `id` | `<dir>.<name>` | Unique block identifier following `<collection>.<block>` convention |
| `version` | `0.1.0` | Semantic version |
| `kind` | `standard` | Block kind: `standard`, `map`, or `reduce` |
| `network` | `false` | Whether the block needs network access at runtime |
| `description` | `""` | Human-readable description (fill this in) |
| `entrypoint` | `<name>` | Name used to resolve the source file |
| `inputs` | `{}` | Input declarations (fill this in) |
| `outputs` | `{}` | Output declarations (fill this in) |

After generating the manifest, you should fill in the `description`, `inputs`, and `outputs` fields, and set `kind` and `network` as appropriate for your block.

### Generated source files

The source file location and content depend on the detected language.

#### Rust

File: `src/<name>.rs`

```rust
/// Block: <name>
pub fn run() {
    // TODO: implement block logic
}
```

A note is printed reminding you to register the module in `src/lib.rs` or `src/main.rs`.

#### Go

File: `<name>.go`

```go
package main

// <name> is the entry point for the <name> block.
func <name>() {
	// TODO: implement block logic
}
```

#### Python

File: `src/<package>/<name>.py` (placed inside the first package directory under `src/`)

```python
"""Block: <name>"""
import yaml


def handler(params):
    """Process inputs and write outputs."""
    # TODO: implement block logic
    pass


if __name__ == "__main__":
    with open("params.yaml") as f:
        params = yaml.safe_load(f)
    handler(params)
```

#### TypeScript

File: `src/<name>.ts`

```typescript
// Block: <name>

export function handler(params: Record<string, unknown>): void {
  // TODO: implement block logic
}
```

#### R

File: `R/<name>.R`

```r
# Block: <name>

library(yaml)

params <- read_yaml("params.yaml")

# TODO: implement block logic

# Write outputs to outputs/ directory
```

## Example

```bash
cd my-collection
spade add normalize
```

Output:

```
  Created blocks/normalize.yaml
  Created src/my_collection/normalize.py
```

## Input and output types

When editing the manifest, use the following types for input and output declarations.

**Valid input types:** `file`, `directory`, `collection`, `string`, `number`, `boolean`

**Valid output types:** `file`, `directory`, `collection`, `json`, `expansion`

Scalar input types (`string`, `number`, `boolean`) are supplied via the pipeline's `args` field rather than from upstream block outputs. File-like input types (`file`, `directory`, `collection`) are resolved from dependency outputs automatically.

## See also

- [`spade init`](/cli/init/) for creating the collection first
- [`spade check`](/cli/check/) for validating the manifest after editing
- [Your First Block](/getting-started/first-block/) for a complete walkthrough
