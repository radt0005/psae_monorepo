# spade-runner

The Spade worker binary (`spade-worker`) plus its supporting Go library.
The runner consumes block-execution jobs from a RabbitMQ queue
(`spade.jobs`), runs each job under the Ubuntu [isolate](https://github.com/ioi/isolate)
sandbox via the shared [`core`](../core/) library, and publishes results
to a second queue (`spade.results`).

Full specification: [`../spec/worker.md`](../spec/worker.md).

## Architecture

```
 scheduler -> spade.jobs  ->  spade-worker  ->  spade.results -> scheduler
                                    |
                                    v
                              core.Execute
                                    |
                                    v
                        isolate (Ubuntu sandbox)
                                    |
                                    v
                             block subprocess
```

The runner is intentionally thin.  Block filesystem setup, input
symlinking, language detection, registry lookups, hashing, and subprocess
execution all live in [`core`](../core/).  This package adds only:

| Package | Purpose |
|---|---|
| `spade_runner` (root) | `Job` message type; invocation ID parsing; dependency-manifest resolution |
| `broker/` | RabbitMQ consumer/publisher + in-memory fakes for tests |
| `worker/` | Single-job orchestration (`Worker.Run`) and the consume→execute→publish→ack loop (`RunLoop`) |
| `cmd/spade-worker/` | The daemon entrypoint with signal handling and reconnect |
| `testutil/` | Shared fixtures — a `hello.hello` Go block, plus `InstallHelloFixture` + `IsolateFriendlyTempDir` helpers |

## Build

```bash
go build -o spade-worker ./cmd/spade-worker
```

## Run

All flags have environment-variable equivalents.  Defaults are spec-compliant.

```bash
./spade-worker \
  --amqp-url amqp://guest:guest@localhost:5672/ \
  --work-root "$HOME/.spade/work" \
  --registry "$HOME/.spade/registry.db"
```

| Flag | Env var | Default | Meaning |
|---|---|---|---|
| `--amqp-url` | `SPADE_AMQP_URL` | `amqp://guest:guest@localhost:5672/` | Broker URL |
| `--work-root` | `SPADE_WORK_ROOT` | `$HOME/.spade/work` | Shared-filesystem root for invocation directories |
| `--registry` | `SPADE_REGISTRY` | `$HOME/.spade/registry.db` | SQLite block registry (populated by `spade install`) |
| `--cache-dir` | `SPADE_CACHE_DIR` | *(disabled)* | Optional block output cache |
| `--prefetch` | *(none)* | `1` | AMQP link credit; spec mandates 1 in production |
| `--shutdown-grace-sec` | *(none)* | `60` | Grace window for in-flight job on SIGTERM |
| `--log-level` | `SPADE_LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error` |
| `--skip-isolate-check` | `SPADE_SKIP_ISOLATE_CHECK=1` | `false` | Skip the isolate-available probe (dev only) |

## Semantics

The runner follows the two-mode failure story from the spec:

- **Block failure** (non-zero exit, missing block, hash mismatch, bad
  expansion manifest): publish a `WorkerResult` with `status: "error"`
  and ack the job.  The scheduler halts that pipeline.
- **Worker failure** (broker dropped, registry unreadable, publish
  confirm lost): do **not** ack.  The broker redelivers the message
  to another worker after its consumer timeout.

Ack always happens **after** the result publish confirm arrives, so a
worker crash between the two causes a harmless redelivery; the
scheduler is required to be idempotent (first result wins).

## Testing

Unit tests run without any external dependencies:

```bash
go test ./...
```

Integration tests build the Go fixture block and run it end-to-end
under real `isolate`.  Enable with the `integration` build tag:

```bash
go test -tags integration ./...
```

Integration test requirements:

- Ubuntu with `isolate` installed on `$PATH`.
- A writable directory at `$HOME/.spade-integration-tests/` (or override
  with `SPADE_TEST_ROOT`).  Tests avoid `/tmp` because some
  isolate installations cannot mount tmpfs paths.

## Dependency on `../core/`

The runner imports the local `core` module via a `replace` directive
in `go.mod`:

```
replace core => ../core
```

Any change that should be shared between the CLI (`spade run`), the
scheduler, and the worker goes into `core`.  This package keeps only
the transport + glue layer.
