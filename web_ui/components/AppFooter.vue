<script setup lang="ts">
const year = new Date().getFullYear();
</script>

<template>
  <footer class="bg-spade-black text-spade-white py-spade-lg">
    <div
      class="max-w-7xl mx-auto px-spade-lg flex items-center justify-between text-sm"
    >
      <div class="flex items-center gap-spade-sm">
        <BrandSpadeLogo :width="24" :height="29" dark />
        <span>&copy; {{ year }} Spade Project.</span>
      </div>
      <span class="text-spade-red text-base">&spades;</span>
    </div>
  </footer>
</template>
