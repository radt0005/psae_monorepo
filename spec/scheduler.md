# Scheduler

The scheduler is the heart of the application, in the sense that it is both eseential and central.  

The scheduler is repsonsible for deciding which block will be executed, when, on which worker.  For a single machine, this is relatively simple, but for many concurrent pipelines and many workers this becomes very complex. 

The simple single pipeline scheduler is responsible for two things: 
1. Making sure that the blocks in a pipeline are executed in the correct order, and
2. Handling map and reduce operations for the pipeline (more on that below)


## Order of Execution

The system should maintain an execution order for the blocks in a pipeline based on the dependencies.  This ensures that the dependencies for a block are in place when they are called.  Furthermore, it tracks the execution state of each block, and therefore "knows" which blocks are ready to be executed now, and what the next block to be executed is. 


## Single Instance Scheduler

This is the simplest scheduler, and allows for scheduling a single pipeline. The more complex schedulers are based on multiples of this design.  This one, though, runs a single pipeline.  It is responsible for tracking the correct execution order of the blocks in that pipeline, which blocks have been executed, and whether there are any blocks that can be run now, and which blocks are still remaining to be executed.  It also handles the Map and Reduce operations



## Map and Reduce

Map and reduce operations allow pipelines to fan out processing across the items in a collection and then gather the results back together.  This is the mechanism for parallel "for each" operations.

Map and reduce are implemented as **blocks** with a special `kind` field in their `block.yaml` manifest (`kind: map` or `kind: reduce`).  This keeps the scheduler decoupled from the filesystem while allowing the expansion logic to run where the data lives (on the worker).

### Overview

1. An upstream block produces a **collection** as output (e.g. a directory of tiles)
2. A **map block** (`kind: map`) runs on a worker, inspects the collection, and writes an **expansion manifest** listing the individual items
3. The worker reports the expansion manifest back to the scheduler
4. The scheduler creates **N invocations** of the downstream block, one per item
5. All N invocations execute in parallel (across available workers)
6. A **reduce block** (`kind: reduce`) collects all N outputs as a collection and produces a combined result

### Map Blocks

A map block is responsible for enumerating a collection and producing an expansion manifest.  The core library provides `core.map.files` for the common case of enumerating files in a directory, but users can write custom map blocks (e.g. paginating a database table, splitting a large file into chunks).

A map block's `block.yaml` declares `kind: map` and outputs a special `expansion` type:

```yaml
id: core.map.files
kind: map
language: rust
entrypoint: map_files

inputs:
  source:
    type: collection

outputs:
  manifest:
    type: expansion
```

When the map block completes, it writes an expansion manifest to its outputs directory:

```yaml
# outputs/manifest/expansion.yaml
items:
  - path: inputs/source/tile_001.tif
    key: tile_001
  - path: inputs/source/tile_002.tif
    key: tile_002
  - path: inputs/source/tile_003.tif
    key: tile_003
```

The `key` field is a human-readable identifier for the item.  The `path` field points to the actual file relative to the map block's working directory.  The worker reads this manifest and reports the item list to the scheduler.

The order of items in the manifest must be **deterministic** for a given input collection to support caching.

### Expansion and Invocation IDs

When the scheduler receives the expansion list from the worker, it creates N invocations of the downstream block.  Each invocation gets an ID derived from the pipeline block ID and an **index vector** -- one integer component per enclosing map context, outermost first:

```
<block_id>[.<i>[.<j>…]]
```

For example, if the downstream block has pipeline ID `019cf4bc-ccc` and there are 3 items:
- `019cf4bc-ccc.0`
- `019cf4bc-ccc.1`
- `019cf4bc-ccc.2`

A block nested under two maps (see "Nested Maps" below) carries two components: `019cf4bc-ccc.1.4` is the invocation processing inner item 4 of outer item 1.

This scheme is:
- **Precise**: you know exactly which item (at every nesting level) an invocation corresponds to
- **Debuggable**: if invocation `.7` fails, you can trace it to the 8th item in the expansion; if `.1.4` fails, to inner item 4 of outer item 1
- **Cache-friendly**: the same block ID + index vector produces the same invocation ID across reruns, enabling cache hits when the input data hasn't changed

At cache time, a hash comparison of the expansion manifest verifies that the cached mapping is still valid.

### Map Context Propagation

Blocks downstream of a map block **inherit the map context** and are also invoked N times.  The scheduler knows which blocks are inside a map context from the dependency graph: after a `kind: map` block, all downstream blocks run N times until a `kind: reduce` block is reached.

The index is consistent through the chain.  If blocks C and D are both in the map context:
- `ccc.2` always feeds into `ddd.2`
- This means you can trace a single item through the entire processing chain

### Broadcasting Non-Mapped Inputs

A block inside a map context may also depend on a block **outside** the map context (e.g. a trained model, a reference dataset).  The non-mapped dependency is simply symlinked into every invocation's `inputs/` directory -- it is **broadcast** to all N invocations.  No special annotation is needed; this works because the worker sets up each invocation's inputs independently.

Example pipeline:
```yaml
  - id: ddd
    name: raster.classify
    inputs:
      - ccc                    # mapped: N invocations
      - block: fff
        output: trained_model  # not mapped: broadcast to all N
    args: {}
```

### Reduce Blocks

A reduce block collects the outputs of all N mapped invocations into a single collection input and produces a combined result.  The core library provides common reduce operations (e.g. raster mosaic, table concatenation, VRT creation), and users can write custom reduce blocks.

A reduce block's `block.yaml` declares `kind: reduce`:

```yaml
id: core.reduce.mosaic
kind: reduce
language: python
entrypoint: run.py

inputs:
  tiles:
    type: collection
    item_type: file
    format: GeoTIFF

outputs:
  mosaic:
    type: file
    format: GeoTIFF
```

The scheduler waits for all N mapped invocations to complete, then presents their outputs to the reduce block as a collection in the `inputs/` directory.  The reduce block processes the collection and writes its output normally.

A reduce block always takes a `collection` as input but can output anything -- a single file, a JSON summary, or even another collection.

### Scheduler Flow (detailed)

```
1. Upstream block A completes → produces a collection output
2. Scheduler dispatches map block M to a worker
3. Worker executes M, which enumerates the collection and writes expansion.yaml
4. Worker reads expansion.yaml and reports item list to scheduler
5. Scheduler creates N invocations for downstream block B:
     B: bbb.0, bbb.1, bbb.2, ...
6. Scheduler creates N invocations for each subsequent block in map context:
     C: ccc.0, ccc.1, ccc.2, ...
     D: ddd.0, ddd.1, ddd.2, ...
7. Invocations are scheduled across workers as normal work units
8. All N invocations of the last mapped block complete
9. Scheduler dispatches reduce block R with collected outputs
10. R completes → pipeline continues normally after the reduce
```

### Nested Maps

Map contexts may be **nested**: a second map block inside an open map context fans each outer item out again.  The essential observations that make this work:

- **The inner map block is itself a mapped block of the outer context.**  If the outer expansion has N₁ items, the inner map runs N₁ times (`M2.0`, `M2.1`, …), and each run produces its **own** expansion.  Item counts may differ per outer index -- the fan-out is *ragged* (outer item 0 might expand to 5 inner items, outer item 1 to 300).
- **The inner reduce is also a mapped block of the outer context.**  It runs once per outer index (`R2.0`, `R2.1`, …), each gathering only the sibling invocations that share its index prefix (`X.0.*` for `R2.0`).  Only the outermost reduce runs once.
- **Contexts have static shape but dynamic instances.**  Which blocks belong to which context (the *context tree*) is derived from the pipeline structure at validation time.  Each **instance** of a nested context -- identified by its index prefix -- gets its own expansion and completion tracking at runtime.

Blocks are entirely nesting-agnostic: a map block enumerates its input and writes `expansion.yaml` exactly as at the top level.  Nesting is implemented in the scheduler (per-instance fan-out and reduce readiness) and the worker (index-prefix-based input resolution).

**Broadcasting generalises by prefix.**  A dependency at context depth *e* feeding a block at depth *d* > *e* is resolved using the first *e* components of the consumer's index vector: depth 0 is a plain broadcast (same file for every invocation), an intermediate depth selects the enclosing instance (e.g. a per-outer-tile reference feeding every inner chunk of that tile).

**Empty instances complete vacuously.**  A map instance that expands to zero items creates no member invocations; its reduce runs immediately with an empty collection.  (This also applies at the top level: a map with zero items does not stall the pipeline.)

**Well-nestedness** is validated by `spade check` (see `pipeline.md` §9.3): every context must be closed by a reduce, a block may not combine outputs of two sibling unclosed contexts, and nesting is limited to a maximum depth (currently 4) to bound multiplicative fan-out (N₁×N₂×…).

**Failure semantics are unchanged**: any failed invocation, at any depth, halts the whole pipeline.

Example flow for a depth-2 pipeline `M1 → M2 → X → R2 → R1` where M1 expands to 2 items and M2's instances expand to 3 and 1 items respectively:

```
M1                          runs once   → expansion [a, b]
M2.0, M2.1                  run twice   → expansions [p,q,r] and [s]
X.0.0, X.0.1, X.0.2, X.1.0  run 4×      (ragged: 3 + 1)
R2.0                        runs when X.0.* complete → gathers 3 outputs
R2.1                        runs when X.1.* complete → gathers 1 output
R1                          runs once when R2.0, R2.1 complete
```

## Multiple Instance Scheduler

This system is responsible for running many concurrent pipelines on many workers.  Workers do not share a filesystem; data flowing between blocks moves through object storage (see `worker.md`).  This system maintains a Single Instance Scheduler for each pipeline that is being run, and then assigns block executions to workers.  These workers then handle the execution of the block, and then notify the scheduler of the status (successful execution or error)

This allows for the fair and efficient execution of multiple pipelines across multiple workers.

## State Management

The scheduler's state must be reconstructable from durable storage at any time.  The source of truth for the scheduler is:

- The pipeline DAGs stored in PostgreSQL
- The invocation result history stored in PostgreSQL
- The outstanding work held in the RabbitMQ queues (`spade.jobs` and `spade.results`)

In-memory bookkeeping -- per-pipeline execution state, pending block readiness, in-flight invocation tracking -- is a **cache rebuilt on startup**, not a source of truth.

This requirement allows the scheduler to be restarted (for deploys, crashes, or platform maintenance) without coordinated shutdown, lease handover, or external orchestration.  On startup, the scheduler:

1. Reads the active pipelines and their current execution state from PostgreSQL
2. Reconstructs each pipeline's dependency graph and identifies which blocks are ready, in-flight, complete, or failed
3. Begins consuming from `spade.results` and idempotently applies any results already in the queue (see `worker.md`: idempotency is keyed by invocation ID, so duplicates from in-flight messages at the time of restart are safely ignored)
4. Resumes dispatching ready blocks to `spade.jobs`

The broker's at-least-once delivery and the scheduler's idempotent result consumer already cover messages in flight at the time of restart.  No additional coordination is required between the scheduler and workers during a restart.

## Error Handling

If a block fails (non-zero exit code), the scheduler **halts the pipeline** that the failed block belongs to.  No further blocks in that pipeline are scheduled for execution.  Other concurrently running pipelines are not affected.  The scheduler records the failure and the worker preserves the block's logs for debugging.

