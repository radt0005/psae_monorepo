<script setup lang="ts">
const { isAuthenticated, loading } = useAuth();
const route = useRoute();
const isAuthRoute = computed(() =>
  ["/login", "/signup", "/forgot-password", "/confirm"].includes(route.path),
);
</script>

<template>
  <div class="min-h-screen flex flex-col font-body text-spade-black bg-spade-white">
    <template v-if="loading && !isAuthRoute">
      <main class="flex-1 flex items-center justify-center">
        <p class="text-spade-gray">Loading…</p>
      </main>
    </template>
    <template v-else-if="isAuthenticated">
      <AppHeader />
      <main class="flex-1">
        <slot />
      </main>
      <AppFooter />
    </template>
    <template v-else-if="isAuthRoute">
      <main class="flex-1">
        <slot />
      </main>
    </template>
    <template v-else>
      <Login />
    </template>
  </div>
</template>
