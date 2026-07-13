+++
title = "spade login"
description = "Authenticate to the cloud registry."
weight = 8
+++

The `spade login` command authenticates the CLI to the cloud registry. It is a prerequisite for [`spade publish`](/cli/publish/) and any other command that acts on your behalf against the registry.

## Usage

```bash
spade login
```

```bash
spade logout
```

No arguments are required for either command.

## What it does

### 1. Start the OAuth-style flow

`spade login` initiates an OAuth-style authentication flow against the system's identity provider, **Better Auth**. Depending on your terminal environment, this opens a browser window for you to sign in, or prints a URL and a short code to enter manually.

### 2. Complete authentication in the browser

You sign in (or confirm an existing session) in the browser. Better Auth redirects back to a local callback that the CLI is listening on, handing off a session token.

### 3. Store the session locally

The CLI stores the resulting session credentials under `~/.spade/auth/`. The session is per-user: once stored, subsequent commands that need to talk to the registry as you (like `spade publish`) reuse it without prompting again.

## Logging out

```bash
spade logout
```

Clears the stored credentials from `~/.spade/auth/`. Commands that require authentication will prompt you to run `spade login` again.

## Who needs this

`spade login` is a **developer-only** concern. It authenticates a human, via Better Auth, for actions like publishing a collection. It's used by:

- [`spade publish`](/cli/publish/) — required before submitting a collection
- Other authenticated CLI commands as they're added

**Workers do not use `spade login`.** A worker fleet authenticates to the registry's fetch endpoints with a separate, rotated **service token** provisioned at worker setup, scoped to read-only access. That token is unrelated to any developer's session and is never stored under `~/.spade/auth/`.

## Example

```bash
spade login
```

```
Opening https://auth.spade.dev/authorize?... in your browser.
Waiting for authentication...
Logged in as you@example.com.
```

```bash
spade logout
```

```
Logged out. Cleared session from ~/.spade/auth/.
```

## See also

- [`spade publish`](/cli/publish/) for the command this session is used by
- [`spade setup`](/cli/setup/) for initializing the rest of the local `~/.spade/` directory
