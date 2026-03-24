<script setup lang="ts">
import type { Node, Edge, Connection } from '@vue-flow/core'  
import { VueFlow } from '@vue-flow/core'

// these components are only shown as examples of how to use a custom node or edge
// you can find many examples of how to create these custom components in the examples page of the docs

const flow = useFlow();
const editorTarget = ref<string | null>(null);
const showBlock = ref(false);
const showData = ref(false);
const showImport = ref(false);
const showExport = ref(false);

const handleDataButtonClick = () => {
  showData.value = true;
}

const handleBlockButtonClick = () => {
  showBlock.value = true;
}

const handleExportButtonClick = () => {
  showExport.value = true;
}

const handleImportButtonClick = () => {
  showImport.value = true;
}

const onConnect = (params: Connection) => {
  flow.connect(params.source, params.target);
}

const handleEdit = (data: {id: string}) => {
  console.log(`Editing: ${JSON.stringify(data)}`)
  editorTarget.value = data.id
  showBlock.value = true;
}


const closeModals = () => {
  editorTarget.value = null;
  showBlock.value = false;
  showData.value = false;
  showExport.value = false;
  showImport.value = false;
}


const handleDeleteBlock = (data: {id: string}) => {
  if(data.id !== null){

    const confirmation = confirm("Are you sure you would like to delete this block?");
    if(confirmation) {
      console.log(`Attempting to delete block with id ${data.id}`)
      // why does this not change the nodes?

      flow.removeNode(data.id);
      console.log(`Nodes after delete: ${JSON.stringify(flow.nodes)}`)

    }
  }
  editorTarget.value = null;

}
</script>

<template>
    <div class="h-screen w-auto">
      <UButtonGroup class="w-full">
        <UButton @click="handleDataButtonClick" class="flex-grow">Add Data</UButton>
        <UButton @click="handleBlockButtonClick" class="flex-grow">Add Block</UButton>
        <UButton @click="handleImportButtonClick" class="flex-grow">Import</UButton>
        <UButton @click="handleExportButtonClick" class="flex-grow">Export</UButton>
      </UButtonGroup>
      <VueFlow v-model:nodes="flow.nodes" v-model:edges="flow.edges" @connect="onConnect">
        <template #node-block="props">
          <FlowCustomBlock :name="props.data.label" :id="props.id" @edit="handleEdit" @delete="handleDeleteBlock"></FlowCustomBlock>
        </template>
      </VueFlow>
    </div>
    <USlideover v-model="showBlock" >
      <FlowBlockEditor @close="closeModals" :data="false" :target-id="editorTarget"></FlowBlockEditor>
    </USlideover>
    <USlideover v-model="showData">
      <FlowBlockEditor @close="closeModals" :target-id="editorTarget" :data="true"></FlowBlockEditor>
    </USlideover>
    <USlideover v-model="showExport" :ui="{ width: 'max-w-xl' }">
      <Export @close="closeModals" class="w-full"></Export>
    </USlideover>
    <USlideover v-model="showImport"  :ui="{ width: 'max-w-xl' }" >
      <Import @close="closeModals"></Import>
    </USlideover>
</template>

<style>
/* import the necessary styles for Vue Flow to work */
@import '@vue-flow/core/dist/style.css';

/* import the default theme, this is optional but generally recommended */
@import '@vue-flow/core/dist/theme-default.css';
</style>