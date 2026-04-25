import type { Edge } from "@vue-flow/core";
import type {
  BlockManifest,
  EdgeBindings,
  EdgeData,
  PortDecl,
  PortType,
} from "./types";

/**
 * I/O wiring resolution per spec/web_ui.md §"Input/Output Wiring" and
 * spec/pipeline.md §5.4.
 *
 * Pure functions only — the editor wires these up in `useFlow` / Base.vue.
 */

export type ResolvedBinding = {
  /** Output declared on the upstream block manifest. */
  output: string;
  /** Input declared on the downstream block manifest. */
  input: string;
};

export type ResolveOutcome =
  | {
      kind: "ok";
      bindings: ResolvedBinding[];
    }
  | {
      kind: "ambiguous";
      /** All compatible candidates, grouped by downstream input. */
      candidates: Record<string, string[]>;
      /** Inputs and outputs (with descriptions) to surface in the prompt. */
      inputDescriptions: Record<string, string | undefined>;
      outputDescriptions: Record<string, string | undefined>;
      /** Suggested partial bindings (any unambiguous matches we *could* lock). */
      partial: ResolvedBinding[];
    }
  | {
      kind: "invalid";
      reason: string;
    };

/** True if two ports are compatible at the type level. */
export function portsCompatible(out: PortDecl, inp: PortDecl): boolean {
  // Scalars must match exactly on type. file/directory/json compatible by type.
  // collection compatible if either both are collection (item_type matched if
  // both declared) or — per spec/scheduler.md reduce semantics — a collection
  // input may absorb a non-collection output via expansion, but that's the
  // worker/scheduler's job. The UI is conservative: same primary type.
  if (out.type !== inp.type) return false;
  if (out.type === "collection" && out.item_type && inp.item_type) {
    return out.item_type === inp.item_type;
  }
  return true;
}

/**
 * Resolve which upstream outputs feed which downstream inputs given two
 * manifests and any explicit bindings the user has already locked in.
 */
export function resolveConnection(
  upstream: BlockManifest,
  downstream: BlockManifest,
  preExistingExplicit: ResolvedBinding[] = [],
): ResolveOutcome {
  const remainingOutputs = new Map<string, PortDecl>(
    Object.entries(upstream.outputs ?? {}),
  );
  const remainingInputs = new Map<string, PortDecl>(
    Object.entries(downstream.inputs ?? {}),
  );

  const bindings: ResolvedBinding[] = [];

  // Step 1: explicit bindings the user (or YAML) already locked in.
  for (const b of preExistingExplicit) {
    const out = remainingOutputs.get(b.output);
    const inp = remainingInputs.get(b.input);
    if (!out) {
      return {
        kind: "invalid",
        reason: `Output "${b.output}" does not exist on ${upstream.id}.`,
      };
    }
    if (!inp) {
      return {
        kind: "invalid",
        reason: `Input "${b.input}" does not exist on ${downstream.id}.`,
      };
    }
    if (!portsCompatible(out, inp)) {
      return {
        kind: "invalid",
        reason: `Output "${b.output}" (${portLabel(out)}) is not compatible with input "${b.input}" (${portLabel(inp)}).`,
      };
    }
    bindings.push(b);
    remainingOutputs.delete(b.output);
    remainingInputs.delete(b.input);
  }

  // Step 2: pair off unambiguous type matches.
  const pendingInputs = [...remainingInputs.entries()];
  for (const [inputName, inputDecl] of pendingInputs) {
    const matches = [...remainingOutputs.entries()].filter(([, out]) =>
      portsCompatible(out, inputDecl),
    );
    if (matches.length === 1) {
      const [outputName] = matches[0];
      bindings.push({ output: outputName, input: inputName });
      remainingOutputs.delete(outputName);
      remainingInputs.delete(inputName);
    }
  }

  // Step 3: anything still pending is either ambiguous or unsatisfiable.
  if (remainingInputs.size === 0) {
    return { kind: "ok", bindings };
  }

  const candidates: Record<string, string[]> = {};
  const anyAmbiguous = [...remainingInputs.entries()].some(
    ([inputName, inputDecl]) => {
      const matches = [...remainingOutputs.entries()]
        .filter(([, out]) => portsCompatible(out, inputDecl))
        .map(([name]) => name);
      candidates[inputName] = matches;
      return matches.length > 1;
    },
  );

  if (!anyAmbiguous) {
    // Some input has no candidates left — the connection cannot be saturated.
    // We do NOT treat this as invalid because an input may also be filled by
    // another upstream block (or by `args`); we just return what we resolved.
    return { kind: "ok", bindings };
  }

  return {
    kind: "ambiguous",
    candidates,
    inputDescriptions: pickDescriptions(downstream.inputs),
    outputDescriptions: pickDescriptions(upstream.outputs),
    partial: bindings,
  };
}

function pickDescriptions(ports: Record<string, PortDecl>) {
  const out: Record<string, string | undefined> = {};
  for (const [name, decl] of Object.entries(ports ?? {})) {
    out[name] = decl.description;
  }
  return out;
}

function portLabel(p: PortDecl) {
  return p.format ? `${p.type}:${p.format}` : p.type;
}

/**
 * Editor glue: invoke the resolver against a freshly-created edge, and
 * apply any unambiguous bindings to the edge's `data` field. Returns a
 * status the caller uses to decide whether to open the resolution modal,
 * roll back the edge (invalid types), or do nothing (already resolved).
 *
 * Manifests are loaded from the block service.
 */
export async function resolveAndApply(
  flow: { nodes: any[]; edges: any[]; setEdgeData: Function; getNameById: Function },
  source: string,
  target: string,
  loadManifest: (name: string) => Promise<BlockManifest | null> = defaultLoadManifest,
): Promise<"resolved" | "ambiguous" | "invalid" | "unknown"> {
  const upstreamName = flow.getNameById(source);
  const downstreamName = flow.getNameById(target);
  if (!upstreamName || !downstreamName) return "unknown";

  const [upstream, downstream] = await Promise.all([
    loadManifest(upstreamName),
    loadManifest(downstreamName),
  ]);
  if (!upstream || !downstream) return "unknown";

  const outcome = resolveConnection(upstream, downstream);
  if (outcome.kind === "invalid") {
    return "invalid";
  }
  if (outcome.kind === "ambiguous") {
    flow.setEdgeData(source, target, {
      bindings: outcome.partial,
      unresolved: true,
    } satisfies EdgeData);
    return "ambiguous";
  }
  flow.setEdgeData(source, target, {
    bindings: outcome.bindings,
    unresolved: false,
  } satisfies EdgeData);
  return "resolved";
}

/** Default manifest loader — fetches from the Nuxt server. */
async function defaultLoadManifest(name: string): Promise<BlockManifest | null> {
  try {
    return await $fetch<BlockManifest>(`/api/blocks/${encodeURIComponent(name)}`);
  } catch {
    return null;
  }
}

/** True if any edge in the list still needs user resolution. */
export function hasUnresolvedEdges(edges: Edge[] | { value: Edge[] }) {
  const list = (edges as any).value ?? edges;
  return (list as Edge[]).some((e) => (e.data as EdgeData | undefined)?.unresolved);
}

/** Stable list of bindings for ambiguous-edge UIs. */
export function listEdgeBindings(edge: Edge): EdgeBindings {
  return ((edge.data as EdgeData | undefined)?.bindings ?? []) as EdgeBindings;
}
