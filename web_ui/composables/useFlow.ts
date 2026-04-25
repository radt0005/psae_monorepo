import type { Node, Edge } from "@vue-flow/core";
import YAML from "yaml";
import { defineStore } from "pinia";
import { ref } from "vue";
import { v7 } from "uuid";
import type { Block, Pipeline, EdgeData, InputRef } from "~/utils/types";

/**
 * Editor store. Every node carries the spec block invocation id (UUIDv7) as
 * its Vue Flow `id`, so the YAML, the cache, and the editor agree on the same
 * identity end-to-end.
 */
export const useFlow = defineStore("flow", () => {
  const nodes = ref<Node[]>([]);
  const edges = ref<Edge[]>([]);
  /** Pipeline metadata stays separate from nodes so it survives a clear(). */
  const pipelineMeta = ref<{
    name: string;
    version: string;
    description?: string;
  }>({
    name: "untitled-pipeline",
    version: "0.1.0",
  });

  function clear() {
    nodes.value = [];
    edges.value = [];
  }

  function setMeta(meta: Partial<typeof pipelineMeta.value>) {
    pipelineMeta.value = { ...pipelineMeta.value, ...meta };
  }

  /** Add a fresh node from a block-type pick in the editor. Returns the id. */
  function addNode(opts: {
    name: string;
    label: string;
    args?: Record<string, any>;
    x?: number;
    y?: number;
    id?: string;
  }) {
    const id = opts.id ?? v7();
    nodes.value.push({
      id,
      type: "block",
      position: {
        x: opts.x ?? Math.random() * 400,
        y: opts.y ?? 100 + Math.random() * 400,
      },
      data: {
        label: opts.label,
        name: opts.name,
        args: opts.args ?? {},
      },
    });
    return id;
  }

  function removeNode(id: string) {
    nodes.value = nodes.value.filter((n) => n.id !== id);
    edges.value = edges.value.filter((e) => e.source !== id && e.target !== id);
  }

  function updateNode(
    id: string,
    label: string,
    name: string,
    args: Record<string, any>,
  ) {
    const idx = nodes.value.findIndex((n) => n.id === id);
    if (idx < 0) return false;
    nodes.value[idx] = {
      ...nodes.value[idx],
      data: { ...nodes.value[idx].data, label, name, args },
    };
    return true;
  }

  function getArgsById(id: string) {
    return nodes.value.find((n) => n.id === id)?.data?.args ?? null;
  }

  function getNameById(id: string) {
    return nodes.value.find((n) => n.id === id)?.data?.name ?? null;
  }

  /**
   * Add an edge between two nodes. Edge id is composed of source/target so
   * dedupe is a string comparison. Bindings (Phase 2) come in via
   * `setEdgeBindings`.
   */
  function connect(source: string, target: string) {
    const id = `${source}->${target}`;
    if (edges.value.some((e) => e.id === id)) return;
    edges.value.push({
      id,
      source,
      target,
      data: { unresolved: false } satisfies EdgeData,
    });
  }

  function disconnect(source: string, target: string) {
    const id = `${source}->${target}`;
    edges.value = edges.value.filter((e) => e.id !== id);
  }

  function setEdgeData(source: string, target: string, data: EdgeData) {
    const idx = edges.value.findIndex(
      (e) => e.source === source && e.target === target,
    );
    if (idx < 0) return;
    edges.value[idx] = { ...edges.value[idx], data };
  }

  function getEdgeData(source: string, target: string): EdgeData | undefined {
    return edges.value.find((e) => e.source === source && e.target === target)
      ?.data as EdgeData | undefined;
  }

  /** Import a spec-conformant pipeline YAML and replace the editor state. */
  function yamlToNodes(pipelineYaml: string) {
    const pipe = YAML.parse(pipelineYaml) as Pipeline;
    if (!pipe || !Array.isArray(pipe.blocks)) {
      throw new Error("Pipeline YAML must contain a `blocks` array.");
    }

    clear();
    pipelineMeta.value = {
      name: pipe.name ?? "untitled-pipeline",
      version: pipe.version ?? "0.1.0",
      description: pipe.description,
    };

    pipe.blocks.forEach((b, i) => {
      nodes.value.push({
        id: b.id,
        type: "block",
        position: {
          x: 80 + (i % 4) * 240,
          y: 80 + Math.floor(i / 4) * 160,
        },
        data: {
          label: b.name,
          name: b.name,
          args: b.args ?? {},
        },
      });
    });

    pipe.blocks.forEach((b) => {
      // Group references by upstream block so each upstream→downstream pair
      // becomes a single edge whose bindings list the explicit outputs.
      const grouped = new Map<string, EdgeData["bindings"]>();
      const bare = new Set<string>();
      for (const ref of b.inputs ?? []) {
        if (typeof ref === "string") {
          bare.add(ref);
        } else {
          const list = grouped.get(ref.block) ?? [];
          list.push({ output: ref.output, input: "" });
          grouped.set(ref.block, list);
        }
      }
      for (const upstream of bare) {
        connect(upstream, b.id);
      }
      for (const [upstream, bindings] of grouped) {
        connect(upstream, b.id);
        setEdgeData(upstream, b.id, { bindings, unresolved: false });
      }
    });
  }

  /** Build a Pipeline object from the current editor state. */
  function exportPipelineObject(): Pipeline {
    const blocks: Block[] = nodes.value.map((n) => ({
      id: n.id,
      name: n.data?.name ?? n.data?.label ?? "",
      inputs: [],
      args: n.data?.args ?? {},
    }));
    const byId = new Map(blocks.map((b) => [b.id, b]));

    for (const edge of edges.value) {
      const downstream = byId.get(edge.target);
      if (!downstream) continue;
      const data = edge.data as EdgeData | undefined;
      if (data?.bindings && data.bindings.length > 0) {
        for (const binding of data.bindings) {
          downstream.inputs.push({
            block: edge.source,
            output: binding.output,
          } satisfies InputRef);
        }
      } else {
        downstream.inputs.push(edge.source satisfies InputRef);
      }
    }

    return {
      id: v7(),
      name: pipelineMeta.value.name,
      version: pipelineMeta.value.version,
      description: pipelineMeta.value.description,
      blocks,
    };
  }

  return {
    nodes,
    edges,
    pipelineMeta,
    clear,
    setMeta,
    addNode,
    removeNode,
    updateNode,
    getArgsById,
    getNameById,
    connect,
    disconnect,
    setEdgeData,
    getEdgeData,
    yamlToNodes,
    exportPipelineObject,
  };
});
