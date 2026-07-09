import { getMetadata } from "./metadata.ts";
import { buildFunctionArgs } from "./scanning.ts";
import { readBlockManifest, writeOutputs } from "./output.ts";
import { loadSecrets } from "./secrets.ts";

// Runs a block handler and writes its outputs. Synchronous handlers are handled
// synchronously (the returned value is undefined) so existing callers and tests
// are unaffected. Handlers that return a Promise — network blocks and any other
// async, I/O-bound work — are awaited before outputs are written, and `run`
// returns a Promise the caller should await.
export function run(fn: Function): void | Promise<void> {
  loadSecrets(); // scrub SPADE_SECRETS from the environment early
  const metadata = getMetadata(fn);
  const args = buildFunctionArgs(metadata);

  let filteredArgs: Record<string, unknown>;
  if (metadata) {
    const declaredInputs = new Set(Object.keys(metadata.inputs));
    filteredArgs = {};
    for (const [key, value] of Object.entries(args)) {
      if (declaredInputs.has(key)) {
        filteredArgs[key] = value;
      }
    }
  } else {
    filteredArgs = args;
  }

  const result = fn(filteredArgs);
  const manifestOutputs = readBlockManifest();

  if (result !== null && typeof result === "object" && typeof (result as { then?: unknown }).then === "function") {
    return (result as Promise<unknown>).then((resolved) => {
      writeOutputs(resolved, manifestOutputs);
    });
  }

  writeOutputs(result, manifestOutputs);
}
