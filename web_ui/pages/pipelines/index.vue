<script setup lang="ts">
type PipelineRow = {
  id: string;
  name: string;
  description: string | null;
  version: string;
  visibility: "private" | "shared" | "public";
  ownerId: string;
  updatedAt: string;
};

const tabs = [
  { key: "mine", label: "Mine" },
  { key: "shared", label: "Shared with me" },
  { key: "public", label: "Public" },
];
const activeTab = ref<"mine" | "shared" | "public">("mine");

const { data, refresh, pending } = await useFetch<{
  pipelines: PipelineRow[];
}>("/api/pipelines", {
  query: { scope: activeTab },
  default: () => ({ pipelines: [] }),
  watch: [activeTab],
});

const pipelines = computed(() => data.value?.pipelines ?? []);

const flow = useFlow();

const handleOpen = async (id: string) => {
  const row = await $fetch<PipelineRow & { yaml: string }>(
    `/api/pipelines/${id}`,
  );
  flow.yamlToNodes(row.yaml);
  await navigateTo("/editor");
};

const handleDelete = async (id: string) => {
  if (!confirm("Delete this pipeline? This cannot be undone.")) return;
  await $fetch(`/api/pipelines/${id}`, { method: "DELETE" });
  await refresh();
};

const visibilityBadge = (v: PipelineRow["visibility"]) => {
  switch (v) {
    case "public":
      return { color: "green", label: "Public" };
    case "shared":
      return { color: "blue", label: "Shared" };
    default:
      return { color: "gray", label: "Private" };
  }
};
</script>

<template>
  <div class="max-w-6xl mx-auto px-spade-lg py-spade-xl">
    <div class="flex items-center justify-between mb-spade-lg">
      <h1 class="font-heading text-3xl font-bold">Pipelines</h1>
      <UButton color="primary" to="/editor" icon="i-heroicons-plus">
        New pipeline
      </UButton>
    </div>

    <UTabs
      :items="tabs"
      :model-value="tabs.findIndex((t) => t.key === activeTab)"
      @change="(i) => (activeTab = tabs[i].key as any)"
      class="mb-spade-md"
    />

    <p v-if="pending" class="text-spade-gray">Loading…</p>

    <p
      v-else-if="pipelines.length === 0"
      class="text-spade-gray text-center py-spade-xl"
    >
      No pipelines here yet.
    </p>

    <div
      v-else
      class="grid gap-spade-md spade-tablet:grid-cols-2 spade-desktop:grid-cols-3"
    >
      <UCard
        v-for="row in pipelines"
        :key="row.id"
        class="hover:shadow-card-hover transition-shadow"
      >
        <template #header>
          <div class="flex items-start justify-between gap-spade-sm">
            <div>
              <h3 class="font-heading text-lg font-bold">{{ row.name }}</h3>
              <p class="text-xs text-spade-gray">v{{ row.version }}</p>
            </div>
            <UBadge :color="visibilityBadge(row.visibility).color" variant="subtle">
              {{ visibilityBadge(row.visibility).label }}
            </UBadge>
          </div>
        </template>
        <p class="text-sm text-spade-gray-dark line-clamp-2">
          {{ row.description || "No description" }}
        </p>
        <template #footer>
          <UButtonGroup>
            <UButton size="sm" @click="handleOpen(row.id)">Open</UButton>
            <UButton
              size="sm"
              variant="outline"
              color="red"
              @click="handleDelete(row.id)"
              :disabled="activeTab !== 'mine'"
            >
              Delete
            </UButton>
          </UButtonGroup>
        </template>
      </UCard>
    </div>
  </div>
</template>
