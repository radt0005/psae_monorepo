<script setup lang="ts">
const cards = [
  {
    title: "New Pipeline",
    description: "Open the editor and build a workflow from scratch.",
    icon: "i-heroicons-plus-circle",
    to: "/editor",
    cta: "Start building →",
  },
  {
    title: "Browse Pipelines",
    description: "Open, run, or share a pipeline you saved earlier.",
    icon: "i-heroicons-rectangle-stack",
    to: "/pipelines",
    cta: "Browse library →",
  },
  {
    title: "Recent Runs",
    description: "View results, logs, and downloads from your recent runs.",
    icon: "i-heroicons-chart-bar-square",
    to: "/results",
    cta: "View results →",
  },
];
</script>

<template>
  <div>
    <!-- Hero — mirrors ../website/sass/_hero.scss layout -->
    <section
      class="relative flex items-center justify-center min-h-[60vh] py-spade-xxl px-spade-lg"
    >
      <div class="text-center max-w-3xl mx-auto relative z-10">
        <BrandSpadeLogo :width="80" :height="98" class="mx-auto mb-spade-lg" />
        <h1 class="font-heading font-bold text-6xl mb-spade-md text-spade-black">
          Spade
        </h1>
        <p class="text-xl text-spade-gray mb-spade-xl leading-relaxed">
          Data processing at scale. Built for geospatial.
        </p>
        <div class="flex gap-spade-md justify-center flex-wrap">
          <UButton size="lg" color="primary" to="/editor">New Pipeline</UButton>
          <UButton size="lg" variant="outline" color="primary" to="/pipelines">
            Browse Pipelines
          </UButton>
        </div>
      </div>

      <!-- Playing-card corner accents (cf. _hero.scss .hero__corner) -->
      <div
        class="absolute top-8 left-8 font-heading text-spade-red opacity-15 leading-none text-center"
      >
        <span class="block text-3xl font-bold">A</span>
        <span class="block text-2xl">&spades;</span>
      </div>
      <div
        class="absolute bottom-8 right-8 rotate-180 font-heading text-spade-red opacity-15 leading-none text-center"
      >
        <span class="block text-3xl font-bold">A</span>
        <span class="block text-2xl">&spades;</span>
      </div>
    </section>

    <section class="max-w-6xl mx-auto px-spade-lg pb-spade-xxl">
      <div
        class="grid gap-spade-lg spade-tablet:grid-cols-2 spade-desktop:grid-cols-3"
      >
        <NuxtLink
          v-for="card in cards"
          :key="card.to"
          :to="card.to"
          class="group relative bg-spade-white border border-[#e0e0e0] rounded-spade-md p-spade-lg shadow-card hover:shadow-card-hover hover:-translate-y-1 transition-all overflow-hidden hover:no-underline"
        >
          <UIcon :name="card.icon" class="text-3xl text-spade-red mb-spade-md" />
          <h3
            class="font-heading text-xl font-bold text-spade-black mb-spade-sm group-hover:text-spade-red transition-colors"
          >
            {{ card.title }}
          </h3>
          <p class="text-spade-gray text-sm leading-relaxed">
            {{ card.description }}
          </p>
          <span
            class="inline-block mt-spade-md text-spade-red font-semibold text-sm group-hover:text-spade-red-light"
          >
            {{ card.cta }}
          </span>
          <!-- corner accent -->
          <span
            class="absolute top-2 left-2 text-xs text-spade-red opacity-30 font-bold leading-none"
          >&spades;</span>
        </NuxtLink>
      </div>
    </section>
  </div>
</template>
