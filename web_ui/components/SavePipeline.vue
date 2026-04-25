<script setup lang="ts">
import YAML from "yaml";
import { PipelineBuilder } from "~/utils/pipeline";
import { hasUnresolvedEdges } from "~/utils/wiring";
import { validatePipeline } from "~/utils/validate";

const emit = defineEmits(["close"]);
const flow = useFlow();

const name = ref(flow.pipelineMeta.name === "untitled-pipeline" ? "" : flow.pipelineMeta.name);
const description = ref(flow.pipelineMeta.description ?? "");
const version = ref(flow.pipelineMeta.version);
const visibility = ref<"private" | "shared" | "public">("private");
const errorMessage = ref<string | null>(null);
const submitting = ref(false);

const visibilityOptions = [
  { label: "Private — only me", value: "private" },
  { label: "Shared — by invitation", value: "shared" },
  { label: "Public — anyone signed in", value: "public" },
];

const handleSave = async () => {
  errorMessage.value = null;
  submitting.value = true;

  try {
    if (!name.value) {
      errorMessage.value = "Name is required.";
      return;
    }
    if (hasUnresolvedEdges(flow.edges)) {
      errorMessage.value = "Resolve ambiguous connections first.";
      return;
    }

    flow.setMeta({
      name: name.value,
      description: description.value || undefined,
      version: version.value,
    });

    const builder = new PipelineBuilder({
      name: name.value,
      version: version.value,
      description: description.value || undefined,
    });
    for (const node of flow.nodes) builder.addBlockFromNode(node);
    for (const edge of flow.edges) builder.parseEdge(edge);
    const pipeline = builder.exportPipeline();

    const result = validatePipeline(pipeline);
    if (!result.ok) {
      errorMessage.value = result.errors.join(" ");
      return;
    }

    const yaml = YAML.stringify(pipeline);
    await $fetch("/api/pipelines", {
      method: "POST",
      body: {
        name: name.value,
        description: description.value || undefined,
        version: version.value,
        yaml,
        visibility: visibility.value,
      },
    });

    emit("close");
    await navigateTo("/pipelines");
  } catch (e: any) {
    errorMessage.value =
      e?.data?.statusMessage || e?.message || "Save failed.";
  } finally {
    submitting.value = false;
  }
};

const handleCancel = () => emit("close");
</script>

<template>
  <UCard>
    <template #header>
      <h3 class="font-heading text-xl font-bold">Save pipeline</h3>
    </template>

    <form class="space-y-spade-md" @submit.prevent="handleSave">
      <UFormGroup label="Name" required>
        <UInput v-model="name" placeholder="my-cool-pipeline" />
      </UFormGroup>
      <UFormGroup label="Description">
        <UTextarea v-model="description" :rows="3" />
      </UFormGroup>
      <UFormGroup label="Version">
        <UInput v-model="version" placeholder="0.1.0" />
      </UFormGroup>
      <UFormGroup label="Visibility">
        <USelect v-model="visibility" :options="visibilityOptions" />
      </UFormGroup>
      <p v-if="errorMessage" class="text-sm text-spade-red">
        {{ errorMessage }}
      </p>
    </form>

    <template #footer>
      <UButtonGroup>
        <UButton color="primary" :loading="submitting" @click="handleSave">
          Save to library
        </UButton>
        <UButton variant="outline" @click="handleCancel">Cancel</UButton>
      </UButtonGroup>
    </template>
  </UCard>
</template>
