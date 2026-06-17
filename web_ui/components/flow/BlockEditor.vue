<script setup lang="ts">
import { v7 } from "uuid";
import type { BlockListItem } from "~/utils/types";

const props = defineProps<{ targetId: string | null }>();
const emit = defineEmits(["close"]);
const flow = useFlow();

const isUpdate = computed(() => props.targetId !== null);
const id = computed(() => props.targetId ?? v7());

const formData = ref<Record<string, any>>({});
const blocks = ref<BlockListItem[]>([]);
const selectedBlock = ref<BlockListItem | undefined>();
const schema = ref<any>(null);

const loadBlockList = async (): Promise<BlockListItem[]> => {
  const res = await $fetch<{ blocks: BlockListItem[] }>("/api/blocks");
  return res.blocks;
};

/**
 * Pull the args form-schema. With the new manifest, scalar inputs become
 * the form fields. We synthesise a JSON-Schema from the inputs so the
 * existing SchemaForm continues to work.
 */
const loadBlockSchema = async (name: string) => {
  const manifest = await $fetch<any>(
    `/api/blocks/${encodeURIComponent(name)}`,
  );
  return manifestToFormSchema(manifest);
};

function manifestToFormSchema(manifest: any) {
  const properties: Record<string, any> = {};
  const required: string[] = [];
  for (const [name, decl] of Object.entries(manifest.inputs ?? {}) as Array<
    [string, any]
  >) {
    if (
      decl.type === "string" ||
      decl.type === "number" ||
      decl.type === "boolean"
    ) {
      properties[name] = {
        type: decl.type,
        description: decl.description,
      };
      required.push(name);
    }
  }
  return { type: "object", properties, required };
}

const { data, status } = await useAsyncData("blocks", loadBlockList);

watch(status, () => {
  if (status.value === "success" && data.value) {
    blocks.value = data.value;
    if (!selectedBlock.value) selectedBlock.value = blocks.value[0];
  }
});

watch(selectedBlock, async () => {
  if (selectedBlock.value?.name) {
    schema.value = await loadBlockSchema(selectedBlock.value.name);
  }
});

onMounted(async () => {
  if (data.value) blocks.value = data.value;

  if (props.targetId !== null) {
    const args = flow.getArgsById(props.targetId);
    const selectedName = flow.getNameById(props.targetId);
    if (selectedName) {
      const found = blocks.value.find((b) => b.name === selectedName);
      if (found) selectedBlock.value = found;
    }
    if (args) formData.value = args;
  }

  if (selectedBlock.value && !schema.value) {
    schema.value = await loadBlockSchema(selectedBlock.value.name);
  }
});

const handleUpdate = (data: any) => {
  formData.value = data;
};

const handleSubmit = () => {
  if (isUpdate.value) {
    flow.updateNode(
      id.value,
      selectedBlock.value?.label ?? "",
      selectedBlock.value?.name ?? "",
      formData.value,
    );
  } else {
    flow.addNode({
      id: id.value,
      label: selectedBlock.value?.label ?? "",
      name: selectedBlock.value?.name ?? "",
      args: formData.value,
    });
  }
  emit("close");
};

const handleCancel = () => emit("close");
</script>

<template>
  <UCard>
    <template #header>
      <p>Block Editor</p>
    </template>
    <USelectMenu
      v-model="selectedBlock"
      :options="blocks"
      searchable
      :search-attributes="['name', 'label']"
      searchable-placeholder="Filter blocks by name…"
      placeholder="Select a block"
      class="text-black"
    />
    <p
      v-if="selectedBlock?.network"
      class="text-amber-700 text-sm mt-2"
      title="This block requires network access at runtime."
    >
      ⚠ Requires network access
    </p>
    <p v-if="selectedBlock?.description" class="text-sm text-gray-600 mt-2">
      {{ selectedBlock.description }}
    </p>
    <div v-if="schema">
      <SchemaForm
        :schema="schema.parameters ? schema.parameters : schema"
        :targetId="props.targetId"
        @update:modelValue="handleUpdate"
      />
    </div>
    <template #footer>
      <UButton class="m-4" :onclick="handleSubmit">Submit</UButton>
      <UButton class="m-4" :onclick="handleCancel">Cancel</UButton>
    </template>
  </UCard>
</template>
