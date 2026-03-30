+++
title = "Secure by Default"
weight = 5
description = "Every block runs in an isolated sandbox with restricted filesystem and network access."
template = "features/page.html"
+++

Spade takes security seriously. Every block executes inside a sandbox that limits what it can access.

## Process Isolation

Each block runs as an independent subprocess inside an `isolate` sandbox. This provides:

- **Filesystem containment** — Blocks can only access their invocation working directory
- **Process isolation** — Blocks cannot interfere with each other or the host system
- **Resource limits** — Memory and CPU usage are constrained per block

## Network Access

Network access is **disabled by default**. Blocks that need network connectivity must explicitly declare it in their manifest:

```yaml
network: true
```

This opt-in model ensures that data processing blocks can't make unexpected network calls unless authorized.

## Integrity Verification

Spade computes a content hash of each block at install time. Before execution, the worker verifies the hash to detect any post-install tampering. If the hash doesn't match, execution is halted.

## Trust Model

- Blocks from trusted repositories are verified via content hash
- The sandbox prevents untrusted code from escaping its boundaries
- Network declarations are visible in the manifest for audit
- All block execution is logged for traceability
