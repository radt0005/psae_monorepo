+++
title = "spade secret"
description = "Manage local secrets for pipeline runs."
weight = 9
+++

The `spade secret` command manages **local** secrets — database connection strings, API keys, and other sensitive values — used by `spade run`. Values are stored in the operating-system keychain and injected into blocks at run time, so they never appear in a pipeline file, logs, or on disk.

A pipeline block references a secret by name in its `secrets` mapping. The block author reads it with `get_secret("<logical-name>")`; the pipeline binds that logical name to one of your stored secrets:

```yaml
- id: 019cf4bc-2222-7000-0000-000000000000
  name: db.query
  args:
    query: "SELECT * FROM parcels"
  secrets:
    db: prod-postgres-dsn   # value is the NAME of a stored secret, never the secret
```

## Usage

```bash
spade secret set <name>     # store a value (prompted, or piped via stdin)
spade secret list           # list stored secret names
spade secret rm <name>      # remove a secret
```

## Setting a secret

The value is read from the terminal without echoing:

```bash
spade secret set prod-postgres-dsn
Enter secret value: ················
Stored secret "prod-postgres-dsn" in the OS keychain.
```

Or piped from standard input, for scripting:

```bash
echo "$DATABASE_URL" | spade secret set prod-postgres-dsn
```

## Storage

Secrets are kept in the OS keychain:

| Platform | Backend |
|----------|---------|
| macOS | Keychain |
| Windows | Credential Manager |
| Linux | Secret Service (libsecret) |

## Local vs. cloud secrets

Local secrets (this command) and cloud secrets (stored in the Spade KMS and used by cloud runs) are **independent namespaces**. This is deliberate: the same pipeline can resolve `prod-postgres-dsn` to a development database locally and a production database in the cloud, without editing the pipeline. A block that references a secret must find it in the local keychain to run locally.

## Errors

If a pipeline block references a secret that is not in the keychain, `spade run` fails with an actionable message naming the secret and how to set it:

```
resolving secrets for block db.query: secret "prod-postgres-dsn" (bound to "db")
is not in the local keychain; set it with `spade secret set prod-postgres-dsn`
```
