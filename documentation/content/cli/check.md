+++
title = "spade check"
description = "Validate block collections or pipeline files."
weight = 4
+++

The `spade check` command validates block collections and pipeline files. It operates in two modes depending on whether you pass a pipeline file as an argument.

## Usage

```bash
# Validate the block collection in the current directory
spade check

# Validate a pipeline YAML file
spade check <pipeline.yaml>
```

## Collection validation

When run without arguments, `spade check` validates all block manifests in the current directory's `blocks/` folder. The current directory must be a block collection root (containing a language marker file).

### Checks performed

1. **Required fields.** Every manifest must have `id`, `version`, `inputs`, and `outputs` fields.

2. **Valid input types.** Each declared input must have a type from the allowed set: `file`, `directory`, `collection`, `string`, `number`, `boolean`.

3. **Valid output types.** Each declared output must have a type from the allowed set: `file`, `directory`, `collection`, `json`, `expansion`.

4. **Block ID convention.** The `id` field must follow the `<collection>.<block>` dotted naming convention.

5. **Entrypoint existence.** The entrypoint (or the block name if `entrypoint` is not set) must resolve to an existing source file. The expected file path depends on the language:

   | Language | Expected path |
   |----------|--------------|
   | Rust | `src/<entrypoint>.rs` |
   | Go | `<entrypoint>.go` |
   | Python | `src/<package>/<entrypoint>.py` |
   | TypeScript | `src/<entrypoint>.ts` |
   | R | `R/<entrypoint>.R` |

6. **Map block constraint.** Blocks with `kind: map` must declare at least one output with `type: expansion`.

7. **Reduce block constraint.** Blocks with `kind: reduce` must declare at least one input with `type: collection`.

### Example: collection validation

```bash
cd my-collection
spade check
```

Success output:

```
Collection is valid. 3 block(s) checked.
```

Failure output:

```
Collection validation failed with 2 error(s):
  - block normalize: missing required field 'version'
  - block transform: entrypoint file src/my_collection/transform.py does not exist
```

## Pipeline validation

When given a pipeline YAML file as an argument, `spade check` loads the file, looks up each referenced block in the local registry, and runs a series of structural and type-compatibility checks.

### Checks performed

1. **Unique block IDs.** All `id` values within the pipeline must be unique.

2. **Valid input references.** Every invocation ID referenced in a block's `inputs` array must correspond to another block in the same pipeline.

3. **Known block types.** Every `name` value must refer to a block type that is installed in the local registry (`~/.spade/registry.db`).

4. **Acyclic dependency graph.** The dependency graph formed by input references must be a directed acyclic graph (DAG). Cycles are detected via topological sort.

5. **Input/output type compatibility.** For each block, the input resolution algorithm checks that upstream block outputs can be matched to downstream block inputs by type. Explicit references (with `block` and `output` keys) are resolved first, then bare references are matched by type. Ambiguous matches (one output type matching multiple unmatched inputs) are reported as errors.

6. **Required arguments.** Scalar inputs (`string`, `number`, `boolean`) declared in a block's manifest must have corresponding entries in the pipeline block's `args` map.

7. **Map/reduce constraints.** Map blocks must have an expansion output, must eventually be followed by a reduce block downstream, and must not have a nested map block before the corresponding reduce. Reduce blocks must accept a collection input.

### Lockfile side effect

If the pipeline contains short codes (`@<identifier>`) instead of UUIDs, `spade check` resolves them against a sibling lockfile named `<pipeline-stem>.lock.yaml` (for `pipeline.yaml`, the lockfile is `pipeline.lock.yaml`). The first run creates the lockfile and assigns a fresh UUIDv7 to each short code. Subsequent runs reuse those bindings, so caching continues to work across reruns.

```
Wrote /path/to/pipeline.lock.yaml
Pipeline 'satellite-reproject' is valid.
```

If the lockfile is corrupt or holds an invalid UUID, the command exits with status 1 and prints:

```
invalid lockfile: binding "@reproject" in pipeline.lock.yaml is not a valid UUID: ...
To regenerate the lockfile from scratch, delete /path/to/pipeline.lock.yaml.
```

See [Short Codes and Hand-Authoring](/pipelines/short-codes/) for the full reference.

### Example: pipeline validation

```bash
spade check reproject-pipeline.yaml
```

Success output:

```
Pipeline "reproject-example" is valid.
```

Failure output:

```
Pipeline validation failed with 2 error(s):
  - block 019cf4bc-1111-7000-0000-000000000000 references non-existent block 019cf4bc-9999-7000-0000-000000000000
  - block 019cf4bc-2222-7000-0000-000000000000 (raster.reproject): input "raster" (type "file") has no matching source
```

## Exit behavior

If validation succeeds, the command exits with status 0. If any errors are found, they are printed to stderr and the command exits with status 1.

## See also

- [`spade add`](/cli/add/) for creating new block manifests
- [`spade install`](/cli/install/) for installing collections into the registry
- [`spade run`](/cli/run/) which also validates the pipeline before execution
