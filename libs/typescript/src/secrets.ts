// Access to secrets injected into a block by the Spade runtime.
//
// Secrets are delivered as the SPADE_SECRETS environment variable — a JSON
// object mapping the block's logical secret names to their values — by the
// worker (cloud) or CLI (local). This module parses that blob, serves values
// through getSecret, and scrubs the variable from the environment so it is not
// inherited by any subprocess the block spawns. See spec/secrets.md §4.

let cache: Record<string, string> | null = null;

// loadSecrets parses and caches SPADE_SECRETS, removing it from the environment
// on first read. Idempotent.
export function loadSecrets(): Record<string, string> {
  if (cache === null) {
    const raw = process.env.SPADE_SECRETS;
    delete process.env.SPADE_SECRETS;
    cache = raw ? (JSON.parse(raw) as Record<string, string>) : {};
  }
  return cache;
}

/**
 * Return the secret bound to a logical `name` for this block.
 *
 * The mapping from logical name to a stored secret is declared in the pipeline
 * (spec/secrets.md §3.2); the value is injected by the worker (cloud) or CLI
 * (local). Throws if the name was not provided — a declared-but-unresolved
 * secret is a real error, not empty.
 */
export function getSecret(name: string): string {
  const secrets = loadSecrets();
  if (!Object.prototype.hasOwnProperty.call(secrets, name)) {
    throw new Error(
      `secret "${name}" was not provided to this block; ` +
        "declare it in the pipeline's secrets mapping",
    );
  }
  return secrets[name]!;
}

// Test-only: reset the module cache so a test can inject a fresh SPADE_SECRETS.
export function _resetSecretsForTest(): void {
  cache = null;
}
