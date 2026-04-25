<script setup lang="ts">
const route = useRoute();
const targetId = route.params.id?.toString() || "test";
</script>

<template>
  <div class="max-w-3xl mx-auto px-spade-lg py-spade-xxl">
    <h1 class="font-heading text-3xl font-bold mb-spade-md">Run {{ targetId }}</h1>
    <UCard class="border-l-4 border-l-spade-yellow">
      <template #header>
        <p class="font-semibold">Results viewing is offline</p>
      </template>
      <p class="text-spade-gray-dark">
        Run results will be available once the cloud scheduler ships. Until
        then, view your saved pipelines in the
        <NuxtLink to="/pipelines" class="text-spade-red font-semibold"
          >library</NuxtLink
        >.
      </p>
    </UCard>
  </div>
</template>
