<script setup lang="ts">
import type { BlockListItem } from "~/utils/types";

const { data, pending } = await useFetch<{ blocks: BlockListItem[] }>(
  "/api/blocks",
  { default: () => ({ blocks: [] }) },
);

const search = ref("");
const kindFilter = ref<"all" | "standard" | "map" | "reduce">("all");
const networkOnly = ref(false);

const filtered = computed(() => {
  const list = data.value?.blocks ?? [];
  return list.filter((b) => {
    if (kindFilter.value !== "all" && b.kind !== kindFilter.value) return false;
    if (networkOnly.value && !b.network) return false;
    if (search.value) {
      const needle = search.value.toLowerCase();
      const haystack = `${b.name} ${b.label} ${b.description ?? ""}`.toLowerCase();
      if (!haystack.includes(needle)) return false;
    }
    return true;
  });
});

const collections = computed(() => {
  const set = new Set<string>();
  for (const b of data.value?.blocks ?? []) {
    const dot = b.id.indexOf(".");
    if (dot > -1) set.add(b.id.slice(0, dot));
  }
  return Array.from(set).sort();
});

const kindOptions = [
  { value: "all", label: "All kinds" },
  { value: "standard", label: "Standard" },
  { value: "map", label: "Map" },
  { value: "reduce", label: "Reduce" },
];
</script>

<template>
  <div class="max-w-6xl mx-auto px-spade-lg py-spade-xl">
    <h1 class="font-heading text-3xl font-bold mb-spade-md">Blocks</h1>
    <p class="text-spade-gray mb-spade-lg">
      Browse the installed block catalogue. Click a block to see its inputs,
      outputs, and example usage.
    </p>

    <div
      class="flex flex-wrap items-center gap-spade-md mb-spade-lg"
    >
      <UInput
        v-model="search"
        icon="i-heroicons-magnifying-glass"
        placeholder="Search blocks…"
        class="flex-1 min-w-64"
      />
      <USelect v-model="kindFilter" :options="kindOptions" class="w-44" />
      <UCheckbox
        v-model="networkOnly"
        label="Network-required only"
      />
    </div>

    <p v-if="pending" class="text-spade-gray">Loading…</p>
    <p
      v-else-if="filtered.length === 0"
      class="text-spade-gray text-center py-spade-xl"
    >
      No blocks match these filters.
    </p>

    <div
      v-else
      class="grid gap-spade-md spade-tablet:grid-cols-2 spade-desktop:grid-cols-3"
    >
      <NuxtLink
        v-for="block in filtered"
        :key="block.id"
        :to="`/blocks/${encodeURIComponent(block.name)}`"
        class="block bg-spade-white border border-[#e0e0e0] rounded-spade-md p-spade-md shadow-card hover:shadow-card-hover hover:-translate-y-0.5 transition-all hover:no-underline relative"
      >
        <div class="flex items-start justify-between mb-spade-sm">
          <h3 class="font-heading text-lg font-bold text-spade-black">
            {{ block.label }}
          </h3>
          <div class="flex gap-1">
            <UBadge
              :color="block.kind === 'map' ? 'blue' : block.kind === 'reduce' ? 'green' : 'gray'"
              variant="subtle"
              size="xs"
            >
              {{ block.kind }}
            </UBadge>
            <UBadge
              v-if="block.network"
              color="amber"
              variant="subtle"
              size="xs"
              title="Requires network access"
            >
              net
            </UBadge>
          </div>
        </div>
        <code class="text-xs text-spade-gray block mb-spade-sm">{{ block.id }}</code>
        <p class="text-sm text-spade-gray-dark line-clamp-3">
          {{ block.description || "No description" }}
        </p>
      </NuxtLink>
    </div>
  </div>
</template>
