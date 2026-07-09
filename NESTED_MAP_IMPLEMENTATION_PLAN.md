# Nested Map/Reduce — Implementation Plan

> **Status: implemented.**  All six phases landed; `test/pipelines/base_map_nested.yaml`
> runs a depth-2 pipeline end-to-end locally, and the full local integration
> suite (97 checked / 76 run) passes.

This plan removes the "no nested maps" restriction (`spec/scheduler.md` §Nested
Maps, `spec/pipeline.md` §9.3 rule 2) and replaces it with support for
arbitrarily nested map/reduce contexts, up to a configurable depth limit.
Depth 2 is the essential target; the design below is depth-N from the start
because nothing about depth 2 is cheaper to build.

The block interface does **not** change.  A map block enumerates its input and
writes `expansion.yaml` exactly as today; it never knows whether it is nested.
All changes live in the scheduler, the worker's input setup, validation, and
the ID scheme.

```
outer map M1 ──► inner map M2 ──► work X ──► inner reduce R2 ──► outer reduce R1
   runs 1×          runs N₁×       runs ΣNᵢ×      runs N₁×            runs 1×

invocation IDs:   M2.0, M2.1, …    X.0.0 … X.0.4, X.1.0 … X.1.299    R2.0, R2.1, …
                  (one expansion    (ragged: item counts vary
                   per instance)     per outer index)
```

---

## 0. Design Decisions

- **Index vectors, dot-joined.**  An invocation is identified by
  `<uuid>[.<i>[.<j>[…]]]` — one integer component per enclosing map context,
  outermost first.  This extends the existing `<uuid>.<i>` scheme, so
  object-storage keys (`outputs/<invocation_id>/…`), cache keys, and log paths
  generalize without a new naming convention.
- **Context instances, not just contexts.**  A map context remains a *static*
  region of the DAG (map block → … → reduce block).  What becomes dynamic is
  the **instance**: each index prefix under which a nested context runs has its
  own expansion manifest, its own item count, and its own completion tracking.
  Fan-out is **ragged** — inner instance counts vary per outer index.
- **The inner map and inner reduce are themselves mapped blocks** of the outer
  context.  `M2` runs N₁ times, each run producing its own expansion.  `R2`
  runs N₁ times, each gathering only its own instance's siblings (`X.<i>.*`).
  Only the outermost reduce runs once.
- **Well-nestedness is validated, not assumed.**  Every block gets a *context
  path* (the chain of map blocks enclosing it).  Every edge must be: within one
  context, parent→child (a broadcast into the nested context), the map block's
  entry edge, or the reduce block's exit edge.  Anything else — inner block
  feeding an outer block directly, two maps merging into one reduce, a reduce
  closing a map it isn't nested under — is rejected by `spade check`.
- **Depth limit.**  `core.MaxMapDepth = 4`.  Invocation counts multiply
  (N₁×N₂×…); a hard cap turns a pathological pipeline into an authoring-time
  error instead of a scheduler meltdown.  Constant, trivially raised later.
- **Empty expansions complete instantly.**  A map instance that expands to
  zero items marks that context instance complete; its reduce runs with an
  empty collection.  (Today `HandleMap` returns early on zero items and the
  pipeline stalls forever — that bug gets fixed in Phase 3 and applies at
  depth 1 too.)
- **Failure semantics unchanged.**  Any failed invocation halts the whole
  pipeline (`spec/scheduler.md` §Error Handling).  No partial-instance retry
  or cleanup logic.
- **Shared context-path computation.**  The context tree is computed in
  `core` from the pipeline + manifests, and used identically by the scheduler
  (fan-out), the validator (`spade check`), and the worker (input resolution).
  The `Job` message already carries the full pipeline and manifests
  (`runner/types.go` `InvocationFromJob`), so **no message-schema change** is
  needed for the worker to know each dependency's depth.

---

## 1. Order of Work

Each phase compiles and passes tests on its own; depth-1 behavior is preserved
throughout (existing `test/pipelines/base_map_*.yaml` keep passing after every
phase).

| Phase | Deliverable | Value |
| --- | --- | --- |
| 1 | Index vectors in types, IDs, messages | mechanical generalization; no behavior change |
| 2 | Context tree + well-nestedness validation | `spade check` accepts nested pipelines, rejects malformed ones |
| 3 | Scheduler: per-instance fan-out and reduce | nested pipelines schedule correctly |
| 4 | Worker/CLI input setup: prefix-aware symlinks | **nested pipelines run end-to-end locally** |
| 5 | Server engine, snapshot, API | nested pipelines run in the cloud path; UI can display fan-out |
| 6 | Spec + docs + integration test pipelines | production-ready |

---

## 2. Phase 1 — Index vectors

Goal: replace the single optional index with a vector everywhere, keeping
depth-1 semantics identical.  `MapIndices == nil` ⇒ not mapped; `len == 1` ⇒
exactly today's behavior.

### 2.1 `core/types.go`
- `BlockInvocation` (`core/types.go:190`): `MapIndex *int` →
  `MapIndices []int` (`yaml:"map_indices,omitempty"`).
- `InvocationID()` (`core/types.go:201`) joins all components:
  ```go
  func (b *BlockInvocation) InvocationID() string {
      id := b.Id.String()
      for _, i := range b.MapIndices {
          id += fmt.Sprintf(".%d", i)
      }
      return id
  }
  ```
- Add the inverse in `core` (moving it up from `runner`):
  `ParseInvocationID(string) (uuid.UUID, []int, error)`, splitting every
  dotted suffix component.  Add `MapDepth()` and `IndexPrefix(d int) string`
  helpers — Phase 3 and 4 key everything off prefixes.

### 2.2 `runner/types.go`
- `ParseInvocationID` (`runner/types.go:52`) becomes a thin wrapper over the
  core version (or is deleted in favor of it); `InvocationFromJob`
  (`runner/types.go:93`) sets `MapIndices`.

### 2.3 Server + CLI touchpoints (mechanical)
- `server/store/types.go:77` — `MapIndex *int` → `MapIndices []int`.  If this
  is persisted (DB column / serialized row), add the migration here; new field
  is additive, old rows have no indices.
- `server/engine/engine.go:492,543,574,680-701` — carry the vector; the
  nil-vs-set branching at 680-701 becomes `len(MapIndices)`-aware in Phase 5.
- `server/api/server.go:210,236,263,316` — JSON field `map_indices`.
- `cli/cmd/run.go:161-175` — progress label `block[i.j]`; cache key
  `__map_index__` → `__map_indices__` (string form of the vector, e.g. `"0.3"`)
  so each nested invocation caches independently.  `cli/cmd/run.go:297-298`
  (peer dir) is generalized properly in Phase 4.
- `core/scheduler_server.go:34,110` — `BlockSnapshot.MapIndex` → `MapIndices`;
  `intPtr` helper retired.

### 2.4 Tests
- Round-trip tests: `InvocationID()` ↔ `ParseInvocationID` for depths 0–3.
- All existing scheduler/executor tests updated mechanically
  (`MapIndex: intPtr(2)` → `MapIndices: []int{2}`).

---

## 3. Phase 2 — Context tree + validation

Goal: compute each block's context path once, in `core`, and use it for
validation.  The scheduler and worker consume the same structure in later
phases.

### 3.1 `core/pipeline.go` (or new `core/map_context.go`)
New shared computation:
```go
// ContextPath is the chain of enclosing map block IDs, outermost first.
// len(path) == the depth at which the block's invocations run.
type ContextPath []uuid.UUID

// BuildContextPaths assigns every block its context path by walking the
// DAG in topological order: a kind:map block's *downstream* context is
// its own path + itself; a kind:reduce block closes the innermost open
// context (its own invocations run at the parent depth).
func BuildContextPaths(p Pipeline, manifests map[string]BlockManifest,
    g DependencyGraph) (map[uuid.UUID]ContextPath, error)
```
Notes:
- A map block *itself* runs at its parent depth (it is a mapped block of the
  enclosing context); blocks strictly between it and its reduce run one
  deeper.  A reduce block runs at the depth of the context it closes minus
  one — i.e. the inner reduce is mapped by the outer context.
- A block reachable from two parents must get a consistent path; conflict ⇒
  validation error (this is the "two maps merge into one reduce" case).

### 3.2 Replace the no-nesting check
- Delete `hasNestedMap` / `checkNestedMap` (`core/pipeline.go:470-499`) and the
  rejection at `core/pipeline.go:420-424`.
- `validateMapReduce` (`core/pipeline.go:387`) instead checks, using the
  context paths:
  1. every `kind: map` block's context is closed by exactly one reduce
     (generalizes `hasDownstreamReduce`, which stays);
  2. **edge legality** — for every edge A→B with paths Pa, Pb: either
     Pa == Pb (same context), Pa is a proper prefix of Pb (broadcast into the
     nested context), or the edge is a map-entry / reduce-exit edge.  Reject
     inner→outer edges that bypass the reduce, and sibling-context edges;
  3. `len(path) <= MaxMapDepth` for every block;
  4. existing rules (map outputs `expansion`, reduce accepts `collection`)
     unchanged.
- Error messages name the offending blocks and say what a legal shape looks
  like ("block X (depth 2, under maps M1→M2) feeds block Y (depth 1); route
  it through M2's reduce instead").

### 3.3 Tests
Table-driven validation tests: legal depth-2 chain, ragged siblings (two inner
map/reduce pairs in parallel branches of one outer context), inner→outer
bypass (reject), merged reduce (reject), depth > MaxMapDepth (reject),
sequential non-nested map/reduce pairs (still legal — unchanged today).

---

## 4. Phase 3 — Scheduler fan-out and reduce per instance

The heart of the change.  `core/scheduler.go` map/reduce handling is rewritten
around context instances.

### 4.1 `MapContext` becomes per-instance (`core/scheduler.go:11`)
```go
type MapContext struct {
    MapBlockID     uuid.UUID
    ParentPath     ContextPath          // enclosing contexts, outermost first
    MappedBlockIDs map[uuid.UUID]bool   // blocks strictly inside this context
    ReduceBlockID  uuid.UUID
    // Expansions holds one item list per *instance* of this context,
    // keyed by index prefix ("" for a top-level map; "0", "1", … when
    // this map is itself nested one deep; "0.2", … two deep).
    Expansions map[string][]ExpansionItem
}
```
`IdentifyMapContexts` / `walkMapContext` (`core/scheduler.go:151-202`) are
rebuilt on `BuildContextPaths` from Phase 2: one `MapContext` per map block,
with `MappedBlockIDs` = blocks whose path ends in this map block, stopping at
this context's reduce (a nested map's subtree belongs to the nested context,
not this one — the nested *map block itself* is in `MappedBlockIDs`).

### 4.2 Bookkeeping keys become invocation-ID strings
`CompletedBlocks` / `PendingBlocks` are keyed by `uuid.UUID` today, with the
`uuid.NewSHA1(blockID, index)` synthetic-key trick for mapped invocations
(`core/scheduler.go:254,302`).  Replace both with maps keyed by the invocation
ID **string** — it already encodes the full vector, it's what the server-side
engine uses (`core/scheduler_server.go:147-149` comment), and it kills the
synthetic-UUID indirection.  `promoteReady` (`core/scheduler.go:103`)
dependency checks then work per-instance by constructing the expected
dependency invocation ID from the dependent's own index prefix (see 4.5).

### 4.3 `HandleMap` (`core/scheduler.go:205`)
When a map result arrives it now carries the *instance* that produced it (the
map invocation's own index vector, e.g. `M2.1`):
1. Store `ctx.Expansions[prefix] = items` where `prefix` is the map
   invocation's index-vector string (`""` for a top-level map).
2. For each block in `ctx.MappedBlockIDs`, create invocations with
   `MapIndices = parentIndices + [j]` for `j < len(items)` — i.e. `X.1.0 …
   X.1.(N₁-1)` when `M2.1` expands.  Blocks whose other dependencies aren't
   complete yet go to `PendingBlocks` under their invocation-ID key.
3. **Zero items**: record the empty expansion and immediately run the
   reduce-readiness check for this instance (4.4) so the instance completes
   instead of stalling.  This fixes the existing early-return bug at
   `core/scheduler.go:206`.
4. A nested map block inside this context is fanned out like any other mapped
   block; each of its runs will later produce its own `ExecutionStatusMap`
   result, re-entering `HandleMap` with a longer prefix.  No special casing.

### 4.4 `HandleReduce` → per-instance readiness (`core/scheduler.go:286`)
Replace the "scan all contexts, count everything" loop with an instance
check invoked whenever a mapped invocation completes:
- For context `ctx` and instance `prefix`: the reduce invocation
  `R.<prefix>` is ready when, for every block B in `ctx.MappedBlockIDs`,
  all `B.<prefix>.<j>` (`j < len(ctx.Expansions[prefix])`) are in
  `CompletedBlocks` — and, for nested maps inside this context, their own
  instances have completed (their reduce `R2.<prefix'>` completing is the
  signal, since `R2 ∈ ctx.MappedBlockIDs`... note: nested *interior* blocks
  are NOT in `ctx.MappedBlockIDs`, so the outer check naturally waits on the
  inner reduce, which is exactly right).
- The reduce invocation is created with `MapIndices = prefix` (empty for the
  outermost reduce, so it runs once as a plain invocation — unchanged from
  today at depth 1).

### 4.5 `promoteReady` (`core/scheduler.go:103`)
For a pending invocation at depth d, a dependency at depth e ≤ d is satisfied
by the completed invocation whose ID is the dependency UUID + the first e
components of the dependent's vector.  Depth comes from the Phase-2 context
paths.  (e == 0 is today's broadcast; e == d is today's peer case.)

### 4.6 Tests (`core/scheduler_test.go`)
Simulation-style tests (feed results in, assert executable set):
- depth-2 happy path with ragged counts (outer 2 items → inner 3 and 1);
- inner reduce runs once per outer index, outer reduce runs once at the end;
- zero-item inner expansion → inner reduce still runs, pipeline completes;
- zero-item **top-level** expansion (regression for the stall bug);
- broadcast from depth 0 and from depth 1 into depth-2 blocks;
- failure of one `X.1.0` halts everything (unchanged semantics);
- sequential map/reduce after a nested pair.

---

## 5. Phase 4 — Worker and CLI input setup

`core/block.go SetupInputSymlinks` (`core/block.go:68`) generalizes by prefix
matching.  The function gains the dependency context depths (computed via
`BuildContextPaths` from the pipeline + manifests already in hand — worker:
from the `Job`; CLI: it has the pipeline locally).

- **Case 1 — consuming from a map block** (`core/block.go:88`): the expansion
  lives in the map block's *instance* dir.  For current vector `v`, source map
  at depth d (its runs use `v[:d]`), read
  `<srcUUID>[.<v[:d]>]/outputs/<out>/expansion.yaml` and take item
  `v[d]` (the next component), not `v[len-1]` blindly.  At depth 1 this is
  exactly today's behavior.
- **Case 2 — reduce gathering siblings** (`core/block.go:117`): the glob
  `srcUUID + ".*"` currently gathers **all** siblings and would silently merge
  every instance's outputs — the one change that corrupts results rather than
  erroring.  Replace with: for reduce instance prefix `p`, match directories
  `srcUUID.<p>.<j>` — exactly one more integer component than `p` (filter with
  `ParseInvocationID`, don't trust the glob).  Also replace
  `sort.Strings(siblingDirs)` with a **numeric** sort on the index — the
  lexicographic sort puts `.10` before `.2` today, breaking the "stable,
  deterministic ordering" promise in `spec/blocks.md` §5.3 for N > 9.  The
  `idxTag_` filename prefix keeps only the final component (collision-free
  within one instance).
- **Case 3/4 — peer and broadcast** (`core/block.go:150`): a dependency at
  depth e resolves to work dir `<srcUUID>` + the first e components of the
  current vector.  Try that dir; the existing "fall back to bare `<srcUUID>`"
  probe disappears in favor of the explicit depth (the probe would pick the
  wrong dir when a broadcast source UUID coincidentally has mapped dirs).
- `SetupBroadcastInputs` (`core/block.go:175`) — fold into the depth-aware
  path; a broadcast is just e < d.
- `cli/cmd/run.go:297-298` — same prefix logic replaces the single-index peer
  construction (share the helper from `core`, don't duplicate).

### Tests
Filesystem-level tests in `core/block_test.go`: build a fake pipeline dir tree
for a depth-2 run (ragged), assert exact symlink sets for: an inner work
block, the inner reduce of each instance, the outer reduce, and a depth-2
block with a depth-0 broadcast and a depth-1 broadcast.

---

## 6. Phase 5 — Server engine, snapshot, API

- `Snapshot()` (`core/scheduler_server.go:105-113`): build `MapInvocations`
  from **all** instances (`ctx.Expansions` iterated in prefix order); add a
  per-map-block `InstanceCounts map[string]int` so the UI can show ragged
  fan-out as aggregates ("M2: 4 instances, 312/340 complete") rather than a
  flat list.
- `server/engine/engine.go:680-701`: the map-fan-out aggregation branches on
  `MapIndex == nil` today; generalize to group by the invocation's base UUID +
  prefix so per-instance progress is trackable.
- `server/api/server.go`: expose `map_indices` and instance counts; keep the
  old `map_index` JSON field emitting `MapIndices[len-1]` for one release if
  the web UI needs a transition window, otherwise drop it in the same PR the
  UI updates.
- Web UI (`web_ui/`): display nested fan-out as per-instance progress under
  the map block node.  Aggregate counts only — no new graph topology is
  needed since contexts are static.

---

## 7. Phase 6 — Spec, docs, integration tests

- `spec/scheduler.md`: replace §Nested Maps with the full model — index
  vectors, context instances, ragged expansion, per-instance reduce, empty
  expansion semantics, `MaxMapDepth`.  Update the "Scheduler Flow (detailed)"
  walkthrough with a depth-2 trace.
- `spec/pipeline.md` §9.3: rule 2 ("no nested maps") becomes the
  well-nestedness rules from Phase 2.
- `spec/worker.md` §Map Block Handling: note that the expansion in a `Result`
  is per map *invocation* (instance), and input resolution is prefix-based.
- `spec/blocks.md`: no contract change; add one sentence stating map blocks
  are nesting-agnostic.
- Integration pipelines in `test/pipelines/`:
  `base_map_nested.yaml` (depth 2, ragged), `base_map_nested_broadcast.yaml`
  (depth-0 and depth-1 broadcasts into the inner context),
  `base_map_nested_empty.yaml` (one outer item expands to zero), plus
  fixtures in `test/generate.py`.  Wire into the existing test-pipeline
  runner.

---

## 8. Explicitly Out of Scope

- Retry/partial-failure semantics inside a context instance (pipeline halt
  stays global).
- Locality-aware dispatch of an instance's invocations to one worker
  (`preferred_worker_id` remains reserved).
- Dynamic/streaming expansion (all items known when the map instance
  completes).
- Cross-run cache sharing of instance expansions beyond the existing
  expansion-hash check (which becomes per-instance for free via invocation-ID
  keying).
