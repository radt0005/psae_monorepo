# Block Caller (Worker)

The Block Caller or Worker process is responsible for turning the output of the scheduler into a block invocation as a subprocess.  This means that it provides a number of functions including security, logging, and execution.

Broadly, the code here is broken into two parts.  There is the core library, and the worker binary.  These worker binaries also handle communication with the server and are responsible for the communication with the scheduling server as well.  

The calling process also makes sure that file inputs are where they need to be by simlinking the block outputs in one directory to the required location in the next blocks inputs folder.  This is all done based on the block schemas and dependencies between blocks. 


## File System
Blocks are called with an invocation ID.  For example, if a block is called with invocation ID "019cf4bc-3695-7985-b3ad-4b3c88a4e04f", then the block would execute with a directory of the same name.  This folder would have four things in it, 
- `019cf4bc-3695-7985-b3ad-4b3c88a4e04f/params.yaml`: This is the parameters supplied by the user in the user interface.  These are basic arguments for the block.
- `019cf4bc-3695-7985-b3ad-4b3c88a4e04f/inputs/<parameter_name>/*`: This subdirectory is the inputs for this block.
- `019cf4bc-3695-7985-b3ad-4b3c88a4e04f/outputs/`: This directory holds the outputs for this block.  Files should be saved here.  
- `019cf4bc-3695-7985-b3ad-4b3c88a4e04f/logs/`: Holds the logs for the block


Based on this layout, when the executor has to call a block, there are some preparations to do. First, the main folder needs to be created.  Then the `params.yaml` file should be written, and the inputs and outputs folders should be created.  The last thing is to create the symbolic links to the inputs.

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

The worker resolves inputs to create the symlinks in the block's `inputs/` directory.  Input references come in two forms:

1. **Explicit references** (with `block` + `output`): The worker symlinks the named output directory from the dependency directly.  These are resolved first.
2. **Bare references** (invocation ID only): The worker matches outputs from the dependency to remaining inputs on the current block using **type matching** against the `block.yaml` manifests.

If type matching is ambiguous (multiple outputs of the same type could satisfy multiple inputs of the same type), the worker rejects the invocation.  This should never happen in practice because `spade check` and the web UI enforce unambiguous mappings at authoring time -- ambiguous cases require explicit references in the pipeline (see `pipeline.md` section 5.4).

## Block Registry

The worker and CLI maintain a SQLite database as a **rebuildable index** of installed block collections.  This database is a performance optimization for fast block lookup -- it is not a source of truth.  The source of truth is always the filesystem at `~/.spade/blocks/`.

### What the registry stores

For each installed block, the registry caches:

- Collection name and version
- Block name and ID
- Language and entrypoint
- Path to the installed collection
- Content hash of the block's binary or entry point script (computed at install time)
- Block manifest metadata (kind, network, input/output declarations)

### Security model

The registry is an **index, not a security boundary**.  The real security boundaries are:

- **Sandbox at runtime**: Blocks are sandboxed and cannot access the registry or the filesystem outside their working directory.  A running block cannot modify the registry.
- **Security screening at upload time**: Collections uploaded to the cloud go through security screening before they are available for installation on production workers.
- **Install-time trust**: The `spade install` command clones a git repository and runs build commands (`cargo build`, `uv sync`, etc.) **unsandboxed** as the current user.  This is the same trust model as `cargo install` or `pip install` -- you are trusting the package author at install time.  For the cloud path, the upload security screening mitigates this.

**Integrity verification**: Before executing a block, the worker computes a hash of the binary or script and compares it against the hash stored in the registry at install time.  If the hashes do not match, the worker refuses to execute the block.  This detects post-install tampering, whether malicious (a compromised process replacing a binary on the shared filesystem) or accidental (partial updates, file corruption).

### Implementation details

- **File permissions**: The database file should be `0600` (readable and writable only by the owner)
- **Concurrent access**: The database should use WAL mode to support concurrent reads from multiple workers and writes from the CLI
- **Rebuilding**: The registry can be rebuilt from the filesystem at any time (e.g. `spade setup --rebuild-index`).  This scans `~/.spade/blocks/`, re-reads all `blocks/*.yaml` manifests, recomputes content hashes, and repopulates the database.
- **Location**: In multi-worker deployments, the registry should live on each worker node (not on the shared filesystem), populated at install time.  This prevents a compromised worker from modifying another worker's registry.

### No encryption needed

The registry contains only block metadata (names, paths, versions, entrypoints, hashes) -- the same information available in the `blocks/*.yaml` files on disk.  It does not store secrets or credentials, so encryption is not required.

## Block Lookup

The worker locates blocks using the registry.  Given a block name like `gdal.rasterize`, the worker:

1. Queries the registry for the collection (`gdal`) and the requested version
2. Retrieves the block manifest metadata and installed path
3. Verifies the content hash of the binary or script against the registry
4. Determines the entrypoint (from the manifest's `entrypoint` field, or defaults to the block name)

## Execution

Block execution depends on the collection's language (detected at install time):

- **Rust / Go**: Call the collection binary with the block name as a subcommand (e.g. `./gdal-tools rasterize`)
- **TypeScript (Bun)**: Call the bundled collection executable with the block name as a subcommand
- **Python**: Call via `uv run <entrypoint>` (the entrypoint may be a named script or a module path)
- **R**: Call via `Rscript <entrypoint>`

For compiled languages and Bun, the collection is a single binary with subcommands.  For Python and R, each block has its own entry point script within the installed package.


## Security

The worker uses the Ubuntu `isolate` package to sandbox block subprocesses.  `isolate` restricts filesystem access, memory, and CPU time for the child process without affecting the worker's main process.  This is critical because the worker must remain unsandboxed to communicate with the scheduler, manage symlinks, read the block registry, and set up invocation directories.

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

## Error Handling

If a block exits with a non-zero exit code, the worker reports the failure to the scheduler.  The scheduler then **halts the pipeline** -- no further blocks in that pipeline are executed.  The logs from the failed block are preserved for debugging.

## Map Block Handling

When the worker executes a `kind: map` block, it performs an additional step after the block completes: it reads the expansion manifest (`outputs/manifest/expansion.yaml`) written by the map block.  The worker then reports the item list back to the scheduler as part of the completion response.

The scheduler uses this item list to create N invocations of the downstream block(s).  The worker is responsible for setting up each mapped invocation's `inputs/` directory with the correct item from the expansion (symlinked from the map block's working directory).

For blocks inside a map context that have dependencies on non-mapped blocks (broadcast inputs), the worker symlinks the non-mapped block's outputs into every mapped invocation's `inputs/` directory.

See `scheduler.md` for the full map/reduce specification.

## Communication

The worker communicates with the scheduler via JSON over HTTP.  The worker polls the scheduler for work, receives block execution assignments, and reports results (success or failure) back to the scheduler upon completion.  For map blocks, the completion response includes the expansion manifest so the scheduler can create the mapped invocations.