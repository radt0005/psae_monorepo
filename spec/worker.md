# Block Caller (Worker)

The Block Caller or Worker process is responsible for turning the output of the scheduler into a block invocation as a subprocess.  This means that it provides a number of functions including security, logging, and execution.

Broadly, the code here is broken into two parts.  There is the core library, and the worker binary.  These worker binaries also handle communication with the server and are responsible for the communication with the scheduling server as well.  

The calling process is also responsible for making each block's file inputs available locally.  It fetches inputs from object storage when needed (or reuses cached copies on the worker's local disk) and symlinks them into the block's inputs folder.  This is all done based on the block schemas and dependencies between blocks. 


## Storage Model

Workers do not share a filesystem.  Each worker has its own local disk, used for two distinct purposes:

1. **Per-invocation scratch**: a working directory for each in-progress invocation (see File System below).  This is short-lived and removed after the invocation completes.
2. **Worker-local input cache**: a content-addressed cache of files fetched from object storage, persisted across invocations.  Reused inputs -- foundational reference data, intermediate outputs of earlier blocks in the same pipeline run -- avoid re-fetch on cache hit.

Data flowing between blocks moves through **object storage**, not the filesystem:

- When a block completes successfully, the worker uploads each output to object storage at a deterministic key derived from the invocation ID and output name (e.g. `outputs/<invocation_id>/<output_name>/...`).
- When the scheduler dispatches a downstream block, the job message identifies the upstream invocations whose outputs feed this block; the worker derives the object-storage keys from those invocation IDs and the input/output declarations.
- On input resolution, the worker consults its local cache first; on cache miss, it fetches the file from object storage and stores it under a content-addressed key in the cache.
- Cache eviction is LRU-bounded by a configured local-disk budget.

This model removes the shared-filesystem dependency.  Workers are stateless beyond their local cache; a worker can be added, removed, or replaced without coordinating with peers.  The cache is purely a performance optimisation -- losing it (fresh worker, disk failure, manual clear) affects latency but not correctness.

Block install paths and the block index also live on per-worker local disk (see Block Index below) but are managed separately from the input cache.

## File System
Blocks are called with an invocation ID.  For example, if a block is called with invocation ID "019cf4bc-3695-7985-b3ad-4b3c88a4e04f", then the block would execute with a directory of the same name.  This folder would have four things in it, 
- `019cf4bc-3695-7985-b3ad-4b3c88a4e04f/params.yaml`: This is the parameters supplied by the user in the user interface.  These are basic arguments for the block.
- `019cf4bc-3695-7985-b3ad-4b3c88a4e04f/inputs/<parameter_name>/*`: This subdirectory is the inputs for this block.
- `019cf4bc-3695-7985-b3ad-4b3c88a4e04f/outputs/`: This directory holds the outputs for this block.  Files should be saved here.  
- `019cf4bc-3695-7985-b3ad-4b3c88a4e04f/logs/`: Holds the logs for the block


Based on this layout, when the executor has to call a block, there are some preparations to do.  First, the invocation directory is created on the worker's local scratch volume.  Then `params.yaml` is written from the job message, and the `inputs/`, `outputs/`, and `logs/` subdirectories are created empty.  For each declared input, the worker either finds the file in the worker-local cache (cache hit) or fetches it from object storage and stores it under a content-addressed key in the cache (cache miss).  The last preparation step is to create the symbolic links from `inputs/<name>/...` to the cached file.

After the block completes (successfully or not), the worker uploads outputs and logs to object storage and removes the local invocation directory.  The upload is the commit point; once it has acked the job message the worker retains no per-invocation state.

Now the executor gets two things to do this job: 
1. The pipeline block specification, and
2. The block Schemas

Each block in the pipeline looks something like this (see `pipeline.md` for the full specification):

```yaml
id: 0197acd7-92a6-7222-b387-2599729a9edc
name: auxdata
inputs:
    - 0197acd5-b635-7222-b387-06ca527c6f5d
    - block: 0197acd6-5145-7222-b387-102e9f7e5ef7
      output: elevation
    - 0197acd6-d81a-7222-b387-1b98b3640f91
args: {}
```

The `id` is the invocation ID, the `name` is the block type to look up, `inputs` are the dependencies, and `args` are the parameters written to `params.yaml`.

The worker resolves each input against the dependencies declared in the job message and creates symlinks in the block's `inputs/` directory.  Input references in the pipeline come in two forms:

1. **Explicit references** (with `block` + `output`): The named output of the named dependency is mapped to a specific input.  These are resolved first.
2. **Bare references** (invocation ID only): The dependency's available outputs are matched to remaining inputs on the current block using **type matching** against the `block.yaml` manifests.

For each resolved input, the worker fetches the corresponding file via the worker-local cache (see Storage Model above) -- a cache hit reuses the file already on disk; a cache miss downloads it from object storage and stores it for future reuse -- and creates a symlink from `inputs/<name>/` to the cached file.

If type matching is ambiguous (multiple outputs of the same type could satisfy multiple inputs of the same type), the worker rejects the invocation.  This should never happen in practice because `spade check` and the web UI enforce unambiguous mappings at authoring time -- ambiguous cases require explicit references in the pipeline (see `pipeline.md` section 5.4).

## Block Index

The worker and CLI maintain a SQLite database as a **rebuildable index** of installed block collections.  This database is a performance optimization for fast block lookup -- it is not a source of truth.  The source of truth on a worker is the filesystem at `~/.spade/blocks/`; the source of truth for what *should* be available in production is the cloud **Plugin Registry** (see `registry.md`), which is a separate component from this local index.

### What the block index stores

For each installed block, the index caches:

- Collection name and version
- Block name and ID
- Language and entrypoint
- Path to the installed collection
- Content hash of the block's binary or entry point script (computed at install time)
- Block manifest metadata (kind, network, input/output declarations)
- Source of the install: registry-fetched (with the artifact's signature recorded) or locally built (developer path)
- Last-verified registry state (`available`, `deprecated`, `yanked`, `recalled`) and the timestamp of the last check

### Security model

The block index is an **index, not a security boundary**.  The real security boundaries are:

- **Sandbox at runtime**: Blocks are sandboxed and cannot access the index or the filesystem outside their working directory.  A running block cannot modify the index.
- **Screening before build at the registry**: For artifacts fetched from the registry, the source is screened before the build runs, and the registry signs the resulting artifact.  See `registry.md` for the full trust chain.
- **Signature verification at install time**: For registry-fetched collections, the worker verifies an ed25519 signature against a list of trusted public keys before unpacking.  Forging an artifact requires the registry's private key, not just access to a worker.
- **Install-time trust for local builds**: The CLI's build-from-source mode (`spade install <git_url>` or `spade install <local_path>`) clones a repository and runs build commands (`cargo build`, `uv sync`, etc.) **unsandboxed** as the current user.  This is the same trust model as `cargo install` or `pip install` -- you are trusting the package author at install time.  Locally-built collections are not signed and the index records them as such; production workers do not use this path.

**Integrity verification**: Before executing a block, the worker computes a hash of the binary or script and compares it against the hash stored in the index at install time.  If the hashes do not match, the worker refuses to execute the block.  This detects post-install tampering, whether malicious (a compromised process replacing a binary on the shared filesystem) or accidental (partial updates, file corruption).

### Implementation details

- **File permissions**: The database file should be `0600` (readable and writable only by the owner)
- **Concurrent access**: The database should use WAL mode to support concurrent reads from multiple workers and writes from the CLI
- **Rebuilding**: The index can be rebuilt from the filesystem at any time (e.g. `spade setup --rebuild-index`).  This scans `~/.spade/blocks/`, re-reads all `blocks/*.yaml` manifests, recomputes content hashes, and repopulates the database.
- **Location**: In multi-worker deployments, the index should live on each worker node (not on the shared filesystem), populated at install time.  This prevents a compromised worker from modifying another worker's index.

### No encryption needed

The block index contains only block metadata (names, paths, versions, entrypoints, hashes) -- the same information available in the `blocks/*.yaml` files on disk.  It does not store secrets or credentials, so encryption is not required.  The worker's registry service token is stored separately and is not part of the index.

## Worker Installer

The worker has its own installation path, separate from the developer-facing `spade install`.  The worker installer is a **fetch-and-unpack** path that requires no language toolchains -- only the runtime libraries and the worker binary.

When the worker is asked to execute a block from a collection it does not have installed (or whose version it does not have installed), it:

1. Queries the Plugin Registry for `<collection>/<version>/<platform>/<arch>`
2. Downloads the artifact tarball and the signature file
3. Verifies the signature against the trusted public keys (fetched from the registry's `/pubkeys` endpoint and refreshed on a schedule)
4. Verifies the artifact's content hash matches the value recorded in the registry metadata
5. Unpacks the tarball into `~/.spade/blocks/<collection>/<version>/`
6. Inserts an entry into the block index, recording the install source (`registry`), the signature, and the registry state at fetch time

If signature or hash verification fails, the worker rejects the artifact, logs the failure, reports it to the scheduler, and does not retry against the same registry endpoint until an operator investigates.

The worker base image contains:

- The worker binary
- Language runtimes: `python3`, `R`, the GDAL system library
- The `isolate` sandbox tool

It does **not** contain build toolchains (`cargo`, `go`, `bun`, `gcc`, `clang`, R/Python development headers).  Those live in the bundler image used by the registry's build step (see `registry.md`).

## Block Lookup

The worker locates blocks using the block index.  Given a block name like `gdal.rasterize`, the worker:

1. Queries the index for the collection (`gdal`) and the requested version
2. If absent, runs the worker installer to fetch and unpack the artifact (see above)
3. Retrieves the block manifest metadata and installed path
4. Verifies the content hash of the binary or script against the index
5. For registry-installed collections, optionally re-checks the registry state if the index entry is stale (see "Recall Handling" below)
6. Determines the entrypoint (from the manifest's `entrypoint` field, or defaults to the block name)

## Recall Handling

The Plugin Registry can mark a collection version as `recalled` (see `registry.md`).  Recalled versions must not execute, even if a worker has the artifact unpacked locally.

When the worker dispatches a block, it consults the local block index.  If the index entry's last-verified registry state is older than a configurable freshness window, the worker re-checks the registry for the current state of that collection version.

If the state is `recalled`, the worker:

1. Refuses to execute the invocation
2. Reports the failure to the scheduler with a "recalled" reason
3. Invalidates the local block index entry for that version
4. Removes the local install from `~/.spade/blocks/<collection>/<version>/`

The scheduler treats this like any other block failure (see "Error Handling") and halts the affected pipeline.  Operationally, security recalls are paired with a worker fleet flush so that no in-flight invocation continues running recalled code; the queue redelivery semantics that make this safe are the responsibility of the scheduler/queue layer, not the worker.

## Execution

Block execution depends on the collection's language (detected at install time):

- **Rust / Go**: Call the collection binary with the block name as a subcommand (e.g. `./gdal-tools rasterize`)
- **TypeScript (Bun)**: Call the bundled collection executable with the block name as a subcommand
- **Python**: Call via `uv run <entrypoint>` (the entrypoint may be a named script or a module path)
- **R**: Call via `Rscript <entrypoint>`

For compiled languages and Bun, the collection is a single binary with subcommands.  For Python and R, each block has its own entry point script within the installed package.


## Security

The worker uses the Ubuntu `isolate` package to sandbox block subprocesses.  `isolate` restricts filesystem access, memory, and CPU time for the child process without affecting the worker's main process.  This is critical because the worker must remain unsandboxed to communicate with the scheduler, manage symlinks, read the block index, and set up invocation directories.

An alternative approach using `go-landlock` was considered but rejected: `go-landlock` applies Landlock restrictions to **all threads** in the process, which means the worker itself would be sandboxed alongside the block.  Since the worker needs full filesystem and network access to perform its orchestration duties, a per-subprocess sandbox like `isolate` is the correct choice.

The sandbox should:
- Restrict the block to its invocation working directory
- Allow execution of required system binaries (e.g. GDAL, Apache Arrow)
- Enforce memory and CPU time limits
- Block network access by default

**Network access**: Blocks do **not** have network access by default.  If a block requires network access (e.g. for calling an LLM API or downloading data), the `block.yaml` must declare `network: true`.  The runtime reads this flag and configures `isolate` accordingly.  The UI should surface which blocks require network access so users can understand the risks.

This security model maintains data security while keeping the system performant (compared to, say, using containers for each block execution).

## Logging

The worker captures `stdout` and `stderr` from the block subprocess and writes them to the `logs/` directory within the block's invocation folder.  Block authors can use standard logging mechanisms (`print` in Python, `cat`/`message` in R, `console.log` in TypeScript, etc.) and the output will be captured automatically.

After the block completes (successfully or with a block failure), the worker uploads the contents of `logs/` to object storage alongside the outputs.  Logs remain accessible for debugging after the local scratch directory is removed.  If the worker process itself fails before this upload step, the logs are lost; the broker will redeliver the job and a fresh execution produces a new log set.

## Telemetry

In addition to capturing block stdout/stderr (see Logging above), the worker emits a structured telemetry record for every invocation.  This telemetry is what makes future scheduling and caching decisions measurable: without it, choices about affinity routing, cache sizing, or worker autoscaling would have no empirical basis.

Each telemetry record must include:

- **Identity**: invocation ID, pipeline ID, block ID, worker ID
- **Wall-clock breakdown**: queue-wait time, input-fetch time, block-execution time, output-upload time, and bookkeeping time (sandbox setup, symlink creation, manifest hashing)
- **Per-input**: source (cache hit on local disk vs. fresh fetch from object storage), bytes transferred, fetch duration
- **Per-output**: bytes written, upload duration
- **Cache state at end of invocation**: hit/miss counts during this invocation, current cache size, evictions performed
- **Outcome**: success, block failure (with exit code), or worker-side failure (with reason)

The record must be emitted from day one, whether or not any consumer is currently reading it.  Transport (structured JSON to stdout, direct write to a telemetry sink, log shipper, etc.) is an implementation choice; the field set above is the contract.

## Error Handling

There are two distinct failure modes the worker treats differently:

1. **Block failure** — the block subprocess exits with a non-zero code.  The block itself ran to completion; the failure is the user's pipeline content.  The worker publishes a `Result` message with `status: "failed"` (including the block's logs and exit code) and **acks the job message**.  The scheduler then halts the affected pipeline -- no further blocks in that pipeline are scheduled.

2. **Worker failure** — the worker process itself crashes, the host loses power, the network drops, or any other condition that prevents the worker from completing the job and publishing a result.  The worker **never acks the job message**.  The broker's consumer timeout elapses and the message is redelivered to another worker, which runs the block fresh.  The scheduler is unaware of the failed attempt.

This split keeps user-pipeline failures (which should halt the pipeline) cleanly separated from infrastructure failures (which should be retried transparently).  Logs from a block failure (mode 1) are uploaded to object storage before the worker acks the job, so they are preserved for debugging.  Logs from a worker failure (mode 2) are lost with the worker; the redelivered job produces a fresh log set on the next attempt.

## Map Block Handling

When the worker executes a `kind: map` block, it performs an additional step after the block completes: it reads the expansion manifest (`outputs/manifest/expansion.yaml`) written by the map block.  The worker then includes the item list in the `Result` message it publishes to `spade.results` (see Communication above).

For **nested** map contexts (see `scheduler.md` §Nested Maps), the inner map block is itself dispatched once per outer item, so the expansion in a `Result` message belongs to that specific map *instance* (`M2.0`, `M2.1`, …), not to the map block as a whole.  The worker needs no special handling for this -- it reads and reports the expansion of whatever invocation it just ran.

Map blocks must **always execute** -- they are excluded from any block-result caching layer, because their expansion items reference files under their own `inputs/` directory, which a restore of `outputs/` alone does not recreate (see `blocks.md` §9).

The scheduler uses each reported item list to create the invocations of the downstream block(s).  Each mapped invocation is dispatched as a normal job; the worker that picks it up resolves inputs by **index prefix**: a dependency at context depth *e* is located under the invocation ID formed by the dependency's block UUID plus the first *e* components of the current invocation's index vector.  A reduce invocation gathers the outputs of every sibling that shares its own index vector plus exactly one more component, in numeric item order.

For blocks inside a map context that depend on non-mapped blocks (broadcast inputs), nothing special is needed at the worker.  A broadcast input is just another input -- fetched from object storage via the cache.  Because the cache is content-addressed, a broadcast input shared across many mapped invocations is fetched once per worker and served from cache for each invocation thereafter.  The same applies to broadcasts from an enclosing context instance (e.g. a per-outer-item reference feeding every inner invocation of that item): the index prefix selects the correct instance.

See `scheduler.md` for the full map/reduce specification.

## Communication

The worker communicates with the scheduler over **RabbitMQ**, using two durable queues:

- `spade.jobs` — **scheduler → worker.** The scheduler publishes one message per block invocation; workers consume as competing consumers.
- `spade.results` — **worker → scheduler.** The worker publishes one message per completed invocation (success *or* failure); the scheduler consumes.

Message bodies are JSON.  Queues are declared `durable: true`, messages are published `persistent: true`, and the broker is configured for at-least-once delivery.

### Job dispatch (`spade.jobs`)

The scheduler publishes a `Job` message for each invocation it wants run.  Workers consume with `prefetch_count = 1` so each worker holds one job at a time; competing-consumers semantics give fair fan-out across N workers with no additional scheduler logic.

A worker **does not ack the job message until the invocation is complete** (successfully or unsuccessfully -- see Error Handling below for the distinction).  This is the core of the failure story: if a worker dies mid-execution, the unacked message is redelivered to another worker once the broker's consumer timeout elapses.  No leases, heartbeats, or dispatch bookkeeping are required on the scheduler side.

The `Job` message schema reserves an optional `preferred_worker_id` field.  It is currently unset by the scheduler and ignored by workers.  The field exists so that locality-aware dispatch -- e.g. routing a downstream invocation to the worker that produced its upstream output -- can be added later by populating the field and introducing per-worker queues, without a message-format migration.  Implementations must accept the field whether present or absent.

### Result reporting (`spade.results`)

Once an invocation finishes, the worker publishes a `Result` message to `spade.results` and only *then* acks the original job message.  The result message carries the invocation ID, final status, and any payload the scheduler needs (log references, output hashes, and -- for map blocks -- the expansion manifest; see Map Block Handling below).

The scheduler must be **idempotent** when consuming results, keyed by invocation ID: in rare cases the worker can publish a result, then crash before acking the job message, causing redelivery and a second attempt whose result the scheduler has already processed.  The simplest scheduler-side treatment is "first result wins, ignore subsequent results for the same invocation."

### Why not HTTP polling

An earlier draft of this spec used HTTP polling for job dispatch.  We switched to RabbitMQ because:

1. **Worker crash recovery is free.**  Ack-on-completion with broker redelivery removes the need for leases, heartbeats, or stuck-job reaping on the scheduler.
2. **Fair dispatch is free.**  Competing consumers with `prefetch_count = 1` removes the need for the scheduler to pick a worker or track worker state.
3. **Backpressure is free.**  Workers that are slow simply hold more unacked messages, and `prefetch_count` caps the in-flight work per worker.

The operational cost of running RabbitMQ is already paid -- the web UI ↔ scheduler path uses a Postgres outbox, but the broker is deployed in the same topology for this worker-side leg -- so the marginal infrastructure cost is zero.