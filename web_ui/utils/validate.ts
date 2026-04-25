import type { BlockManifest, InputRef, Pipeline } from "./types";

/**
 * Client-side mirror of the spec/pipeline.md §7 checks (and a subset of
 * §8.3 for map/reduce). Run on every flowchart change so the user sees
 * problems before they hit `spade check` server-side.
 */

export type ValidationResult =
  | { ok: true; errors: [] }
  | { ok: false; errors: string[] };

export type ValidateOptions = {
  /** Optional manifests keyed by block name. If absent, manifest-aware checks
   *  (type compatibility, required args) are skipped. */
  manifests?: Record<string, BlockManifest>;
};

export function validatePipeline(
  pipeline: Pipeline,
  opts: ValidateOptions = {},
): ValidationResult {
  const errors: string[] = [];
  const blocks = pipeline.blocks ?? [];

  // 1. Unique block ids.
  const seen = new Set<string>();
  for (const b of blocks) {
    if (seen.has(b.id)) errors.push(`Duplicate block id: ${b.id}`);
    seen.add(b.id);
  }

  // 2. All referenced ids exist in the pipeline.
  const idSet = new Set(blocks.map((b) => b.id));
  for (const b of blocks) {
    for (const ref of b.inputs ?? []) {
      const refId = refToId(ref);
      if (!idSet.has(refId)) {
        errors.push(
          `Block "${b.id}" references missing block id "${refId}".`,
        );
      }
    }
  }

  // 3. All `name` values refer to known blocks (when manifests provided).
  const manifests = opts.manifests;
  if (manifests) {
    for (const b of blocks) {
      if (!manifests[b.name]) {
        errors.push(`Block "${b.id}" uses unknown block type "${b.name}".`);
      }
    }
  }

  // 4. DAG is acyclic.
  const cycle = findCycle(blocks);
  if (cycle) errors.push(`Pipeline contains a cycle: ${cycle.join(" → ")}`);

  // 5 + 6. Type compatibility & named-output existence (manifest-aware).
  if (manifests) {
    for (const b of blocks) {
      const downstreamManifest = manifests[b.name];
      if (!downstreamManifest) continue;
      for (const ref of b.inputs ?? []) {
        if (typeof ref === "string") continue;
        const upstream = blocks.find((x) => x.id === ref.block);
        const upstreamManifest = upstream
          ? manifests[upstream.name]
          : undefined;
        if (upstreamManifest && !upstreamManifest.outputs?.[ref.output]) {
          errors.push(
            `Block "${b.id}" references output "${ref.output}" that does not exist on "${upstream?.name}".`,
          );
        }
      }
    }
  }

  // 7. Required args (manifest-aware): an input declared as a scalar
  //    (string/number/boolean) with no upstream binding must be present in args.
  if (manifests) {
    for (const b of blocks) {
      const m = manifests[b.name];
      if (!m) continue;
      for (const [inputName, decl] of Object.entries(m.inputs ?? {})) {
        const isScalar =
          decl.type === "string" ||
          decl.type === "number" ||
          decl.type === "boolean";
        if (!isScalar) continue;
        if (!(inputName in (b.args ?? {}))) {
          errors.push(
            `Block "${b.id}" missing required arg "${inputName}".`,
          );
        }
      }
    }
  }

  // 8. Map/reduce structural checks (subset of pipeline.md §8.3).
  if (manifests) {
    for (const b of blocks) {
      const m = manifests[b.name];
      if (!m) continue;
      if (m.kind === "map") {
        const hasExpansion = Object.values(m.outputs ?? {}).some(
          (o) => o.type === "expansion",
        );
        if (!hasExpansion) {
          errors.push(
            `Map block "${b.id}" must declare an output of type "expansion".`,
          );
        }
      }
      if (m.kind === "reduce") {
        const hasCollection = Object.values(m.inputs ?? {}).some(
          (i) => i.type === "collection",
        );
        if (!hasCollection) {
          errors.push(
            `Reduce block "${b.id}" must declare a "collection" input.`,
          );
        }
      }
    }
  }

  return errors.length
    ? { ok: false, errors }
    : { ok: true, errors: [] };
}

function refToId(ref: InputRef): string {
  return typeof ref === "string" ? ref : ref.block;
}

/** Returns the cycle as a list of block ids if found, otherwise null. */
function findCycle(blocks: Pipeline["blocks"]): string[] | null {
  const adj = new Map<string, string[]>();
  for (const b of blocks) {
    adj.set(
      b.id,
      (b.inputs ?? []).map(refToId),
    );
  }

  const WHITE = 0,
    GRAY = 1,
    BLACK = 2;
  const color = new Map<string, number>();
  for (const id of adj.keys()) color.set(id, WHITE);

  const stack: string[] = [];

  function dfs(node: string): string[] | null {
    color.set(node, GRAY);
    stack.push(node);
    for (const dep of adj.get(node) ?? []) {
      if (!adj.has(dep)) continue;
      const c = color.get(dep);
      if (c === GRAY) {
        // back-edge: extract cycle slice
        const idx = stack.indexOf(dep);
        return stack.slice(idx).concat(dep);
      }
      if (c === WHITE) {
        const found = dfs(dep);
        if (found) return found;
      }
    }
    stack.pop();
    color.set(node, BLACK);
    return null;
  }

  for (const id of adj.keys()) {
    if (color.get(id) === WHITE) {
      const cycle = dfs(id);
      if (cycle) return cycle;
    }
  }
  return null;
}
