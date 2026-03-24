<script setup lang="ts">
const links = [{
  label: 'SPADE',
}, {
  label: 'Home',
  icon: 'i-heroicons-home',
  to: '/'
}, {
  label: 'Create a Pipeline',
  icon: 'i-heroicons-chart-bar',
  to: '/editor'
}, {
  label: 'Results',
  icon: 'i-heroicons-command-line',
  to: '/results'
}]
</script>

<template>
  <UHorizontalNavigation :links="links" class="border-b border-gray-200 dark:border-gray-800" />
</template>