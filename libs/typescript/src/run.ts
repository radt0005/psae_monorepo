import { getMetadata } from "./metadata.ts";
import { buildFunctionArgs } from "./scanning.ts";
import { readBlockManifest, writeOutputs } from "./output.ts";

export function run(fn: Function): void {
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
  writeOutputs(result, manifestOutputs);
}
