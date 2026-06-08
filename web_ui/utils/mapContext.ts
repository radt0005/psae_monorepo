import type { BlockKind } from "./types";

/**
 * Map/reduce context analysis for the flowchart (scheduler.md §"Map and
 * Reduce", pipeline.md §8).
 *
 * A `kind: map` block opens a map context: every block downstream of it runs
 * once per item in the expansion, and stays "in the map context" until a
 * `kind: reduce` block closes it. The reduce gathers the N outputs back into a
 * single collection, so its *outputs* leave the context.
 *
 * This module computes, purely from the dependency graph + each block's kind:
 *  - the map-context stack active at the input ("incoming") and output
 *    ("outgoing") of every block,
 *  - the set of blocks that are inside a map context (for visual grouping),
 *  - "nested map" violations — a map opening while already inside a context,
 *  - which input edges are *broadcast* inputs (a non-mapped dependency feeding
 *    an in-context block; scheduler.md §"map context propagation").
 *
 * It is intentionally dependency-free and side-effect-free so it can be unit
 * tested in isolation and reused server-side if needed.
 */

export interface MapGraphNode {
  id: string;
  kind: BlockKind;
  /** Upstream block ids this block depends on. */
  deps: string[];
}

export interface MapContextResult {
  /** Map-context stack active *at the input of* each block (oldest → newest). */
  incoming: Map<string, string[]>;
  /** Map-context stack active for each block's *outputs*. */
  outgoing: Map<string, string[]>;
  /** Blocks whose incoming context is non-empty (render with grouping). */
  inContext: Set<string>;
  /** Map block ids that open while already inside another map context. */
  nestedMaps: string[];
}

/**
 * Compute map-context membership for a set of graph nodes.
 *
 * The context is modelled as a stack of open map ids. Processing nodes in
 * topological order, a block's incoming stack is the deepest of its
 * dependencies' outgoing stacks (a broadcast dep with an empty/shorter stack
 * never shrinks the context its mapped siblings establish). A `map` pushes its
 * own id; a `reduce` pops the innermost open map.
 */
export function computeMapContext(nodes: MapGraphNode[]): MapContextResult {
  const byId = new Map(nodes.map((n) => [n.id, n]));

  // Kahn topological sort over real (in-graph) dependencies.
  const indeg = new Map<string, number>();
  const children = new Map<string, string[]>();
  for (const n of nodes) {
    indeg.set(n.id, 0);
    children.set(n.id, []);
  }
  for (const n of nodes) {
    for (const d of n.deps) {
      if (!byId.has(d)) continue;
      children.get(d)!.push(n.id);
      indeg.set(n.id, (indeg.get(n.id) ?? 0) + 1);
    }
  }
  const queue = nodes.filter((n) => (indeg.get(n.id) ?? 0) === 0).map((n) => n.id);
  const order: string[] = [];
  while (queue.length) {
    const id = queue.shift()!;
    order.push(id);
    for (const c of children.get(id) ?? []) {
      indeg.set(c, (indeg.get(c) ?? 0) - 1);
      if (indeg.get(c) === 0) queue.push(c);
    }
  }
  // Append any nodes left out by a cycle so they still get (best-effort) entries.
  const placed = new Set(order);
  for (const n of nodes) if (!placed.has(n.id)) order.push(n.id);

  const incoming = new Map<string, string[]>();
  const outgoing = new Map<string, string[]>();
  const nestedMaps: string[] = [];

  for (const id of order) {
    const node = byId.get(id)!;
    // Incoming context = deepest dependency outgoing stack.
    let inc: string[] = [];
    for (const d of node.deps) {
      const o = outgoing.get(d);
      if (o && o.length > inc.length) inc = o;
    }
    incoming.set(id, inc);

    let out = inc.slice();
    if (node.kind === "map") {
      if (inc.length > 0) nestedMaps.push(id);
      out = [...inc, id];
    } else if (node.kind === "reduce") {
      out = inc.slice(0, -1); // close the innermost open map
    }
    outgoing.set(id, out);
  }

  const inContext = new Set<string>();
  for (const [id, inc] of incoming) if (inc.length > 0) inContext.add(id);

  return { incoming, outgoing, inContext, nestedMaps };
}

/**
 * Is the edge `source → target` a broadcast input? True when the target sits
 * inside a map context but the source is outside the innermost open map of
 * that context — i.e. one upstream value fans out to every mapped invocation.
 */
export function isBroadcastEdge(
  result: MapContextResult,
  source: string,
  target: string,
): boolean {
  const inc = result.incoming.get(target);
  if (!inc || inc.length === 0) return false;
  const activeMap = inc[inc.length - 1];
  if (source === activeMap) return false; // the map block itself feeding its context
  const srcOut = result.outgoing.get(source) ?? [];
  return !srcOut.includes(activeMap);
}

/** The innermost open map id for a block, or null if it isn't in a context. */
export function activeMapFor(
  result: MapContextResult,
  id: string,
): string | null {
  const inc = result.incoming.get(id);
  return inc && inc.length > 0 ? inc[inc.length - 1] : null;
}
