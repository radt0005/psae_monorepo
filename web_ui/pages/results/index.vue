<script setup lang="ts">
/**
 * Lists the user's pipeline runs (web_ui.md §"Browsing Results") from the
 * Postgres-backed /api/runs endpoint. Mine / shared-with-me / public tabs
 * mirror the pipeline library.
 */
type RunRow = {
  id: string;
  name: string | null;
  status: "queued" | "running" | "succeeded" | "failed" | "canceled";
  visibility: "private" | "shared" | "public";
  ownerId: string;
  createdAt: string;
  finishedAt: string | null;
};

const tabs = [
  { key: "mine", label: "Mine" },
  { key: "shared", label: "Shared with me" },
  { key: "public", label: "Public" },
];
const activeTab = ref<"mine" | "shared" | "public">("mine");

const { data, refresh, pending } = await useFetch<{ runs: RunRow[] }>(
  "/api/runs",
  {
    query: { scope: activeTab },
    default: () => ({ runs: [] }),
    watch: [activeTab],
  },
);

const runs = computed(() => data.value?.runs ?? []);

// Poll while any visible run is still in flight so status badges advance.
let timer: ReturnType<typeof setInterval> | null = null;
const hasActive = computed(() =>
  runs.value.some((r) => r.status === "queued" || r.status === "running"),
);
watch(
  hasActive,
  (active) => {
    if (active && !timer) {
      timer = setInterval(() => refresh(), 4000);
    } else if (!active && timer) {
      clearInterval(timer);
      timer = null;
    }
  },
  { immediate: true },
);
onUnmounted(() => timer && clearInterval(timer));

const statusBadge = (s: RunRow["status"]) => {
  switch (s) {
    case "succeeded":
      return { color: "green", label: "Succeeded" };
    case "failed":
      return { color: "red", label: "Failed" };
    case "running":
      return { color: "blue", label: "Running" };
    case "canceled":
      return { color: "gray", label: "Canceled" };
    default:
      return { color: "yellow", label: "Queued" };
  }
};

const fmt = (iso: string | null) =>
  iso ? new Date(iso).toLocaleString() : "—";
</script>

<template>
  <div class="max-w-5xl mx-auto px-spade-lg py-spade-xl">
    <div class="flex items-center justify-between mb-spade-lg">
      <h1 class="font-heading text-3xl font-bold">My Runs</h1>
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
      v-else-if="runs.length === 0"
      class="text-spade-gray text-center py-spade-xl"
    >
      No runs here yet. Build a pipeline in the editor and submit it.
    </p>

    <div v-else class="flex flex-col gap-spade-sm">
      <NuxtLink
        v-for="row in runs"
        :key="row.id"
        :to="`/results/${row.id}`"
        class="block"
      >
        <UCard class="hover:shadow-card-hover transition-shadow">
          <div class="flex items-center justify-between gap-spade-md">
            <div class="min-w-0">
              <h3 class="font-heading text-lg font-bold truncate">
                {{ row.name || "Untitled run" }}
              </h3>
              <p class="text-xs text-spade-gray font-mono truncate">
                {{ row.id }}
              </p>
            </div>
            <div class="flex items-center gap-spade-md shrink-0">
              <span class="text-xs text-spade-gray hidden spade-tablet:inline">
                {{ fmt(row.finishedAt || row.createdAt) }}
              </span>
              <UBadge :color="statusBadge(row.status).color" variant="subtle">
                {{ statusBadge(row.status).label }}
              </UBadge>
            </div>
          </div>
        </UCard>
      </NuxtLink>
    </div>
  </div>
</template>
