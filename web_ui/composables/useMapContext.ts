import { computeMapContext, type MapGraphNode } from "~/utils/mapContext";

/**
 * Live map/reduce context analysis for the editor. Derives a dependency graph
 * from the flow store (nodes + edges) and each block's `kind` from the block
 * catalogue, then runs the pure `computeMapContext`. The result is a single
 * shared computed, so every CustomBlock that reads it recomputes only when the
 * graph or catalogue actually changes.
 */
export function useMapContext() {
  const flow = useFlow();
  const catalogue = useBlockCatalogue();

  const result = computed(() => {
    const nodes: MapGraphNode[] = flow.nodes.map((node) => {
      const name = node.data?.name ?? node.data?.label ?? "";
      const kind = catalogue.get(name)?.kind ?? "standard";
      const deps = flow.edges
        .filter((e) => e.target === node.id)
        .map((e) => e.source);
      return { id: node.id, kind, deps };
    });
    return computeMapContext(nodes);
  });

  return { result };
}
