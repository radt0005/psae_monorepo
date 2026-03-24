<script setup lang="ts">

</script>


<template>
  <UContainer>
    <h1 class="prose-xl text-center p-4">Welcome to the Homepage!</h1>
    <UCard class="p-4 m-4">
      <NuxtLink href="/editor">Create a New Workflow</NuxtLink>
    </UCard>
    <UCard class="p-4 m-4">  
      <NuxtLink href="/editor">Edit an Existing Pipeline</NuxtLink>
    </UCard>
    <UCard class="p-4 m-4">
      <NuxtLink href="/results">View results</NuxtLink>
    </UCard>
  </UContainer>
</template>