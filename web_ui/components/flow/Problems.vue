<script setup lang="ts">
import { validatePipeline } from "~/utils/validate";
import { hasUnresolvedEdges } from "~/utils/wiring";
import { PipelineBuilder } from "~/utils/pipeline";

const flow = useFlow();

const errors = computed<string[]>(() => {
  const builder = new PipelineBuilder({
    name: flow.pipelineMeta.name,
    version: flow.pipelineMeta.version,
    description: flow.pipelineMeta.description,
  });
  for (const node of flow.nodes) builder.addBlockFromNode(node);
  for (const edge of flow.edges) builder.parseEdge(edge);
  const result = validatePipeline(builder.exportPipeline());
  const list = result.ok ? [] : result.errors;
  if (hasUnresolvedEdges(flow.edges)) {
    list.push("One or more connections need disambiguation.");
  }
  return list;
});

const open = ref(true);
</script>

<template>
  <div
    v-if="errors.length > 0"
    class="fixed bottom-2 right-2 max-w-md bg-white border border-red-300 rounded shadow z-50"
  >
    <button
      class="w-full text-left px-3 py-2 font-semibold bg-red-50 text-red-700"
      @click="open = !open"
    >
      {{ errors.length }} problem{{ errors.length === 1 ? "" : "s" }}
      <span class="float-right text-xs">{{ open ? "▼" : "▲" }}</span>
    </button>
    <ul v-if="open" class="px-4 py-2 text-sm space-y-1">
      <li v-for="(err, i) in errors" :key="i" class="text-red-800">{{ err }}</li>
    </ul>
  </div>
</template>
