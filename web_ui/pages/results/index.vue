<script setup lang="ts">
/**
 * Results browsing depends on the cloud scheduler (B.5) which is being
 * rebuilt. Once that lands, this page lists the user's runs from the new
 * /api/runs endpoint backed by Postgres.
 */
</script>

<template>
  <div class="max-w-3xl mx-auto px-spade-lg py-spade-xxl">
    <h1 class="font-heading text-3xl font-bold mb-spade-md">My Runs</h1>
    <UCard
      class="border-l-4 border-l-spade-yellow"
    >
      <template #header>
        <p class="font-semibold">Results browsing is offline</p>
      </template>
      <p class="text-spade-gray-dark">
        The cloud scheduler is being rebuilt. Pipelines you save to your
        library will be runnable as soon as it ships. In the meantime, the
        editor, the pipeline library, and the block browser are fully
        functional.
      </p>
      <template #footer>
        <UButtonGroup>
          <UButton color="primary" to="/editor">Open editor</UButton>
          <UButton variant="outline" to="/pipelines">Pipeline library</UButton>
        </UButtonGroup>
      </template>
    </UCard>
  </div>
</template>
