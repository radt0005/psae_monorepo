<script setup lang="ts">
import YAML from "yaml";
import { PipelineBuilder } from "~/utils/pipeline";
import { hasUnresolvedEdges } from "~/utils/wiring";
import { validatePipeline } from "~/utils/validate";

const flow = useFlow();
const pipelineText = ref<string[]>([]);
const validationErrors = ref<string[]>([]);
const emit = defineEmits(["close"]);

const exportPipeline = () => {
  pipelineText.value = [];
  validationErrors.value = [];

  const builder = new PipelineBuilder({
    name: flow.pipelineMeta.name,
    version: flow.pipelineMeta.version,
    description: flow.pipelineMeta.description,
  });
  for (const node of flow.nodes) builder.addBlockFromNode(node);
  for (const edge of flow.edges) builder.parseEdge(edge);

  const pipeline = builder.exportPipeline();
  const result = validatePipeline(pipeline);
  if (!result.ok) {
    validationErrors.value = result.errors;
    return;
  }

  const baseText = YAML.stringify(pipeline);
  pipelineText.value = baseText.split("\n");
};

// Submit the current pipeline as a run (B.5). Posts the resolved YAML to
// /api/runs, which validates, persists a queued run, and enqueues it to the
// scheduler, then routes to the run detail page.
const submitting = ref(false);
const submitError = ref<string | null>(null);

const submitRun = async () => {
  submitError.value = null;
  const yaml = pipelineText.value.join("\n");
  if (!yaml.trim()) return;
  submitting.value = true;
  try {
    const run = await $fetch<{ id: string }>("/api/runs", {
      method: "POST",
      body: { yaml, name: flow.pipelineMeta.name },
    });
    await navigateTo(`/results/${run.id}`);
  } catch (e: any) {
    submitError.value = e?.data?.statusMessage ?? "Submission failed";
  } finally {
    submitting.value = false;
  }
};

const downloadYAML = () => {
  const yamlContent = pipelineText.value.join("\n");
  const blob = new Blob([yamlContent], { type: "text/yaml" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = "pipeline.yaml";
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
};

const handleClose = () => emit("close");
</script>

<template>
  <UCard class="overflow-y-auto">
    <template #header>
      <p>Export as YAML</p>
    </template>
    <div v-if="validationErrors.length > 0" class="mb-4">
      <p class="font-semibold text-spade-red">Pipeline is not valid:</p>
      <ul class="list-disc pl-5 text-spade-red">
        <li v-for="err in validationErrors" :key="err">{{ err }}</li>
      </ul>
    </div>
    <div v-for="text in pipelineText" :key="text">
      <pre>{{ text }}</pre>
    </div>
    <p
      v-if="submitError"
      class="mt-spade-md p-spade-md border-l-4 border-spade-red bg-spade-red-light text-sm"
    >
      {{ submitError }}
    </p>

    <template #footer>
      <UButtonGroup>
        <UButton class="p-3" @click="exportPipeline">Export as YAML</UButton>
        <UButton class="p-3" @click="handleClose">Close</UButton>
        <UButton
          class="p-3"
          @click="downloadYAML"
          :disabled="
            pipelineText.length === 0 ||
            validationErrors.length > 0 ||
            hasUnresolvedEdges(flow.edges)
          "
        >Download</UButton>
        <UButton
          class="p-3"
          color="primary"
          :loading="submitting"
          @click="submitRun"
          :disabled="
            pipelineText.length === 0 ||
            validationErrors.length > 0 ||
            hasUnresolvedEdges(flow.edges)
          "
        >Submit run</UButton>
      </UButtonGroup>
    </template>
  </UCard>
</template>
<style scoped>
pre {
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
