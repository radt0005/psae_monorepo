+++
title = "spade secret"
description = "Manage local and cloud secrets for pipeline runs."
weight = 9
+++

The `spade secret` command manages secrets — database connection strings, API keys, and other sensitive values — used by pipeline blocks. By default it manages **local** secrets: values stored in the operating-system keychain and injected into blocks at run time by `spade run`, so they never appear in a pipeline file, logs, or on disk. With `--remote`, `spade secret set` instead writes to the cloud KMS used by cloud runs, as a CLI convenience for setting a cloud secret without going through the web UI.

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
spade secret set <name>              # store a local value (prompted, or piped via stdin)
spade secret set --remote <name>     # store a value in the cloud KMS instead
spade secret list                    # list stored secret names
spade secret rm <name>               # remove a secret
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

## Setting a remote (cloud) secret

Passing `--remote` targets the cloud KMS instead of the local OS keychain:

```bash
spade secret set --remote prod-postgres-dsn
Enter secret value: ················
Stored secret "prod-postgres-dsn" in the cloud KMS.
```

This is a convenience for setting a cloud secret from the terminal instead of the web UI — useful when scripting environment setup or when you'd rather not leave the CLI. `spade secret list` and `spade secret rm` also accept `--remote` to operate on the cloud namespace. Requires a `spade login` session, since the value is written to your account's cloud secrets.

## Storage

Local secrets (the default, without `--remote`) are kept in the OS keychain:

| Platform | Backend |
|----------|---------|
| macOS | Keychain |
| Windows | Credential Manager |
| Linux | Secret Service (libsecret) |

## Local vs. cloud secrets

Local secrets (the OS keychain, the default) and cloud secrets (the Spade KMS, used by cloud runs, reached with `--remote`) are **independent namespaces**. This is deliberate: the same pipeline can resolve `prod-postgres-dsn` to a development database locally and a production database in the cloud, without editing the pipeline. A block that references a secret must find it in the local keychain to run locally, regardless of whether a same-named secret also exists in the cloud KMS.

## Errors

If a pipeline block references a secret that is not in the keychain, `spade run` fails with an actionable message naming the secret and how to set it:

```
resolving secrets for block db.query: secret "prod-postgres-dsn" (bound to "db")
is not in the local keychain; set it with `spade secret set prod-postgres-dsn`
```
