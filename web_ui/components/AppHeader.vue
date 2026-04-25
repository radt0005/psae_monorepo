<script setup lang="ts">
const route = useRoute();
const auth = useAuth();

const links = [
  { label: "Home", to: "/" },
  { label: "Editor", to: "/editor" },
  { label: "Pipelines", to: "/pipelines" },
  { label: "Blocks", to: "/blocks" },
  { label: "Data", to: "/data" },
  { label: "Results", to: "/results" },
];

const isActive = (to: string) => {
  if (to === "/") return route.path === "/";
  return route.path === to || route.path.startsWith(to + "/");
};

const handleSignOut = async () => {
  await auth.signOut();
  await navigateTo("/login");
};
</script>

<template>
  <header
    class="sticky top-0 z-50 bg-spade-white/95 backdrop-blur border-b border-black/5"
  >
    <nav
      class="flex items-center justify-between max-w-7xl mx-auto h-16 px-spade-lg"
    >
      <NuxtLink
        to="/"
        class="flex items-center gap-spade-sm font-heading text-2xl font-bold text-spade-black hover:no-underline"
      >
        <BrandSpadeLogo :width="28" :height="34" />
        Spade
      </NuxtLink>

      <ul class="hidden spade-tablet:flex items-center gap-spade-lg">
        <li v-for="link in links" :key="link.to">
          <NuxtLink
            :to="link.to"
            class="text-sm font-medium transition-colors relative pb-1"
            :class="
              isActive(link.to)
                ? 'text-spade-red after:content-[\'\'] after:absolute after:left-0 after:right-0 after:-bottom-1 after:h-0.5 after:bg-spade-red'
                : 'text-spade-gray-dark hover:text-spade-red'
            "
          >
            {{ link.label }}
          </NuxtLink>
        </li>
      </ul>

      <div class="flex items-center gap-spade-sm">
        <NuxtLink
          to="/account"
          class="text-sm text-spade-gray-dark hover:text-spade-red"
        >
          {{ auth.user.value?.name || auth.user.value?.email || "Account" }}
        </NuxtLink>
        <UButton
          variant="ghost"
          size="sm"
          icon="i-heroicons-arrow-right-on-rectangle"
          @click="handleSignOut"
        >
          Sign out
        </UButton>
      </div>
    </nav>
  </header>
</template>
