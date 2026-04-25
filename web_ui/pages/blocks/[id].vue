<script setup lang="ts">
import type { BlockManifest } from "~/utils/types";

const route = useRoute();
const name = route.params.id as string;

const { data, pending, error } = await useFetch<BlockManifest>(
  `/api/blocks/${encodeURIComponent(name)}`,
);
</script>

<template>
  <div class="max-w-4xl mx-auto px-spade-lg py-spade-xl">
    <NuxtLink
      to="/blocks"
      class="text-sm text-spade-gray hover:text-spade-red"
    >
      ← All blocks
    </NuxtLink>

    <div v-if="pending" class="mt-spade-lg text-spade-gray">Loading…</div>

    <div v-else-if="error" class="mt-spade-lg text-spade-red">
      Block not found.
    </div>

    <div v-else-if="data" class="mt-spade-md">
      <div class="flex items-center gap-spade-sm mb-spade-md">
        <h1 class="font-heading text-3xl font-bold mb-0">{{ data.id }}</h1>
        <UBadge
          :color="data.kind === 'map' ? 'blue' : data.kind === 'reduce' ? 'green' : 'gray'"
          variant="subtle"
        >
          {{ data.kind }}
        </UBadge>
        <UBadge v-if="data.network" color="amber" variant="subtle">
          requires network
        </UBadge>
      </div>
      <p class="text-spade-gray mb-spade-lg">v{{ data.version }}</p>
      <p
        v-if="data.description"
        class="text-spade-gray-dark mb-spade-xl text-lg"
      >
        {{ data.description }}
      </p>

      <section class="mb-spade-xl">
        <h2 class="font-heading text-xl font-bold mb-spade-md">Inputs</h2>
        <div
          v-if="Object.keys(data.inputs ?? {}).length === 0"
          class="text-spade-gray italic"
        >
          (none)
        </div>
        <div
          v-else
          class="divide-y border border-[#e0e0e0] rounded-spade-md"
        >
          <div
            v-for="(decl, k) in data.inputs"
            :key="k"
            class="p-spade-md"
          >
            <div class="flex items-baseline gap-spade-sm mb-1">
              <code class="font-bold text-spade-red">{{ k }}</code>
              <span class="text-xs text-spade-gray">
                {{ decl.type
                }}<template v-if="decl.format">: {{ decl.format }}</template>
              </span>
            </div>
            <p
              v-if="decl.description"
              class="text-sm text-spade-gray-dark"
            >
              {{ decl.description }}
            </p>
          </div>
        </div>
      </section>

      <section class="mb-spade-xl">
        <h2 class="font-heading text-xl font-bold mb-spade-md">Outputs</h2>
        <div
          v-if="Object.keys(data.outputs ?? {}).length === 0"
          class="text-spade-gray italic"
        >
          (none)
        </div>
        <div
          v-else
          class="divide-y border border-[#e0e0e0] rounded-spade-md"
        >
          <div
            v-for="(decl, k) in data.outputs"
            :key="k"
            class="p-spade-md"
          >
            <div class="flex items-baseline gap-spade-sm mb-1">
              <code class="font-bold text-spade-red">{{ k }}</code>
              <span class="text-xs text-spade-gray">
                {{ decl.type
                }}<template v-if="decl.format">: {{ decl.format }}</template>
              </span>
            </div>
            <p
              v-if="decl.description"
              class="text-sm text-spade-gray-dark"
            >
              {{ decl.description }}
            </p>
          </div>
        </div>
      </section>

      <section>
        <h2 class="font-heading text-xl font-bold mb-spade-md">Example YAML</h2>
        <pre class="bg-spade-black text-spade-white p-spade-md rounded-spade-md overflow-x-auto"><code>- id: 019cf4bc-0000-7000-0000-000000000000
  name: {{ data.id }}
  inputs: []
  args: {}</code></pre>
      </section>
    </div>
  </div>
</template>
