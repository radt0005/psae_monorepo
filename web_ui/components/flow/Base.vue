<script setup lang="ts">
import type { Connection } from "@vue-flow/core";
import { VueFlow } from "@vue-flow/core";

const flow = useFlow();
const catalogue = useBlockCatalogue();
catalogue.load();
const editorTarget = ref<string | null>(null);
const showBlock = ref(false);
const showImport = ref(false);
const showExport = ref(false);
const showSave = ref(false);
const showResolver = ref(false);
const resolverTarget = ref<{ source: string; target: string } | null>(null);

const handleBlockButtonClick = () => {
  editorTarget.value = null;
  showBlock.value = true;
};

const handleExportButtonClick = () => {
  showExport.value = true;
};

const handleImportButtonClick = () => {
  showImport.value = true;
};

const handleSaveButtonClick = () => {
  showSave.value = true;
};

const onConnect = async (params: Connection) => {
  flow.connect(params.source, params.target);
  // Phase 2 wiring: the resolver decides whether the connection is
  // unambiguous, ambiguous (prompt), or invalid.
  const { resolveAndApply } = await import("~/utils/wiring");
  const outcome = await resolveAndApply(flow, params.source, params.target);
  if (outcome === "ambiguous") {
    resolverTarget.value = { source: params.source, target: params.target };
    showResolver.value = true;
  } else if (outcome === "invalid") {
    flow.disconnect(params.source, params.target);
  }
};

const handleEdit = (data: { id: string }) => {
  editorTarget.value = data.id;
  showBlock.value = true;
};

const closeModals = () => {
  editorTarget.value = null;
  showBlock.value = false;
  showExport.value = false;
  showImport.value = false;
  showSave.value = false;
  showResolver.value = false;
  resolverTarget.value = null;
};

const handleDeleteBlock = (data: { id: string }) => {
  if (data.id == null) return;
  if (!confirm("Are you sure you would like to delete this block?")) return;
  flow.removeNode(data.id);
  editorTarget.value = null;
};
</script>

<template>
  <div class="h-screen w-auto">
    <UButtonGroup class="w-full">
      <UButton @click="handleBlockButtonClick" class="flex-grow">Add Block</UButton>
      <UButton @click="handleImportButtonClick" class="flex-grow">Import</UButton>
      <UButton @click="handleSaveButtonClick" class="flex-grow" color="primary">Save</UButton>
      <UButton @click="handleExportButtonClick" class="flex-grow">Export</UButton>
    </UButtonGroup>
    <FlowProblems />
    <VueFlow
      v-model:nodes="flow.nodes"
      v-model:edges="flow.edges"
      @connect="onConnect"
    >
      <template #node-block="props">
        <FlowCustomBlock
          :name="props.data.label"
          :id="props.id"
          @edit="handleEdit"
          @delete="handleDeleteBlock"
        />
      </template>
    </VueFlow>
  </div>
  <USlideover v-model="showBlock">
    <FlowBlockEditor @close="closeModals" :target-id="editorTarget" />
  </USlideover>
  <USlideover v-model="showExport" :ui="{ width: 'max-w-xl' }">
    <Export @close="closeModals" class="w-full" />
  </USlideover>
  <USlideover v-model="showImport" :ui="{ width: 'max-w-xl' }">
    <Import @close="closeModals" />
  </USlideover>
  <USlideover v-model="showSave" :ui="{ width: 'max-w-xl' }">
    <SavePipeline @close="closeModals" />
  </USlideover>
  <UModal v-model="showResolver" :ui="{ width: 'max-w-2xl' }">
    <FlowConnectionResolver
      v-if="resolverTarget"
      :source="resolverTarget.source"
      :target="resolverTarget.target"
      @close="closeModals"
      @cancel="
        () => {
          if (resolverTarget) flow.disconnect(resolverTarget.source, resolverTarget.target);
          closeModals();
        }
      "
    />
  </UModal>
</template>

<style>
@import "@vue-flow/core/dist/style.css";
@import "@vue-flow/core/dist/theme-default.css";
</style>
