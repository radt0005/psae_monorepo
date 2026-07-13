+++
title = "Secrets"
description = "How blocks receive credentials securely with get_secret."
weight = 7
+++

Blocks often need credentials — a database connection string, an AI API key — that must not appear in the pipeline file, logs, or on disk. Spade delivers these through **secrets**: named values a block reads at run time with `get_secret`, resolved from a secure store and injected into the sandbox by the runtime.

## How it works

1. The **block author** reads a secret by a logical name:

   ```python
   from spade import run, get_secret

   def handler():
       dsn = get_secret("db")   # logical name — the credential the block needs
       # ... connect and query ...
   ```

2. The **pipeline** binds that logical name to one of *your* stored secrets. Only the secret's **name** appears in the pipeline — never its value:

   ```yaml
   - id: 019cf4bc-2222-7000-0000-000000000000
     name: db.query
     args:
       query: "SELECT * FROM parcels"
     secrets:
       db: prod-postgres-dsn
   ```

3. At run time the runtime resolves the value — from the OS keychain for a local `spade run`, or from the cloud KMS for a cloud run — and injects it into the block. `get_secret("db")` returns the value.

The value only ever exists in the block process's memory; it is never written to disk.

## `get_secret` in each language

The function is the same across every runtime library — it takes the logical name and returns the value, and raises/returns an error if the secret was not provided.

| Language | Call | Reference |
|----------|------|-----------|
| Python | `get_secret("db")` | [Python handlers](/libraries/python/handlers/#secrets) |
| R | `get_secret("db")` | [R handlers](/libraries/r/handlers/#secrets) |
| TypeScript | `getSecret("db")` | [TypeScript handlers](/libraries/typescript/handlers/#secrets) |
| Go | `GetSecret("db")` | [Go handlers](/libraries/go/handlers/#accessing-secrets) |
| Rust | `get_secret("db")?` | [Rust handlers](/libraries/rust/handlers/#accessing-secrets) |

## Setting secrets

For local runs, store secrets in the OS keychain with the CLI:

```bash
spade secret set prod-postgres-dsn
```

See [`spade secret`](/cli/secret/) for the full command reference.

## Local vs. cloud

Local secrets (keychain) and cloud secrets (KMS) are **independent namespaces**, so the same pipeline can resolve a name to a development credential locally and a production credential in the cloud. A block that references a secret must find it in the store for wherever it runs.

## Network access is separate

Reading a secret does not require network access. If a block also needs the network — for example to reach the database whose connection string it just read — its `block.yaml` must additionally declare `network: true`. The two are independent: `get_secret` works without network, and network access is surfaced separately so users can see which blocks reach out.
