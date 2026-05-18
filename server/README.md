# spade-scheduler

The Spade platform's scheduling server.  Submits jobs to workers via
RabbitMQ (`spade.jobs`), consumes results from workers
(`spade.results`), and persists pipeline state to PostgreSQL.

The full design is in `IMPLEMENTATION_PLAN.md` and the upstream
specifications in `../spec/`.

## Layout

```
server/
├── api/                          Echo v4 HTTP handlers (web UI front door)
├── broker/                       RabbitMQ adapters: JobPublisher / ResultConsumer
├── cmd/spade-scheduler/app/      Binary entrypoint (signal handling, recovery)
├── engine/                       Scheduling engine: drives core.MultiTenantScheduler
├── store/                        Persistence: PgStore (Postgres) + MemStore (tests)
├── testutil/                     Shared test fixtures (currently empty)
├── go.mod / go.sum
├── main.go                       Top-level shim that delegates to cmd/.../app
└── IMPLEMENTATION_PLAN.md
```

## Building

```sh
go build ./...
```

The binary lives at the top-level `server/` (built by `go build .`)
and is also available via `go build ./cmd/spade-scheduler`.

## Running locally

The scheduler dials PostgreSQL and RabbitMQ at startup.  For quick local
iteration on the HTTP API or for tests, the binary supports a
**skip-broker** mode that falls back to an in-memory SQLite store and a
no-op job publisher:

```sh
SPADE_HTTP_ADDR=:1323 SPADE_SKIP_BROKER=1 go run .
```

With a real PostgreSQL + RabbitMQ pair:

```sh
SPADE_HTTP_ADDR=:1323 \
SPADE_DATABASE_URL=postgres://spade:spade@localhost/spade?sslmode=disable \
SPADE_AMQP_URL=amqp://guest:guest@localhost:5672/ \
go run .
```

### Environment variables / flags

| Variable | Flag | Default | Purpose |
|---|---|---|---|
| `SPADE_AMQP_URL` | `-amqp-url` | `amqp://guest:guest@localhost:5672/` | RabbitMQ broker |
| `SPADE_DATABASE_URL` | `-database-url` | *(empty → in-memory SQLite)* | PostgreSQL DSN |
| `SPADE_HTTP_ADDR` | `-http-addr` | `:1323` | HTTP listen address |
| `SPADE_LOG_LEVEL` | `-log-level` | `info` | `debug` / `info` / `warn` / `error` |
| `SPADE_SKIP_BROKER` | `-skip-broker` | unset | When `1`, do not dial RabbitMQ |

## HTTP API

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/` , `/healthz` | Health check |
| `POST` | `/pipelines` | Submit a pipeline (`application/yaml` or JSON `{yaml: "..."}`) |
| `GET` | `/pipelines` | List pipelines (`?status=&submitter=&limit=&offset=`) |
| `GET` | `/pipelines/:id` | Pipeline detail + per-block snapshot |
| `DELETE` | `/pipelines/:id` | Cancel a pipeline |
| `GET` | `/pipelines/:id/invocations` | Persisted invocation history |
| `GET` | `/invocations/:id` | Single invocation row |
| `GET` | `/invocations/:id/logs` | Returns the `logs_path` (caller fetches from object store) |

Optional `X-Spade-User-Id` header records the submitter — Better Auth
integration (`hosting.md` §7) is intentionally deferred.

## Testing

```sh
go test ./...
```

Tests use the in-memory `MemStore`, a fake `JobPublisher`, and a fake
`ResultConsumer`.  No external services required.

## Deployment

The scheduler runs as an App Platform container per `hosting.md` §3.
Build the image with the included `Dockerfile` and push to DO Container
Registry.

State recovery on restart is handled by `engine.Recover(ctx)` per
`scheduler.md` §State Management — pipelines marked `running` in
PostgreSQL are rehydrated and dispatch resumes where it left off.
