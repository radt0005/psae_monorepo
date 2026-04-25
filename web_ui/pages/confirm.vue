<script setup lang="ts">
const auth = useAuth();
const route = useRoute();
const status = ref<"pending" | "ok" | "error">("pending");
const message = ref<string>("");

onMounted(async () => {
  // If the URL carries a verification token, hand it to Better Auth.
  const token =
    (route.query.token as string | undefined) ||
    (route.query.t as string | undefined);
  if (token) {
    try {
      const config = useRuntimeConfig();
      const res = await $fetch(
        `${config.public.betterAuthUrl}/api/auth/verify-email`,
        { method: "POST", body: { token } },
      );
      status.value = "ok";
      message.value = "Your email is verified.";
    } catch (e: any) {
      status.value = "error";
      message.value = e?.message || "Verification failed.";
    }
  } else {
    // No token — just check session and bounce home if logged in.
    await auth.refresh();
    if (auth.isAuthenticated.value) {
      await navigateTo("/");
    }
  }
});
</script>

<template>
  <div
    class="flex flex-col items-center justify-center min-h-screen px-spade-lg bg-spade-white"
  >
    <BrandSpadeLogo :width="48" :height="58" class="mb-spade-lg" />
    <UCard class="max-w-md w-full shadow-card text-center">
      <h2 class="font-heading text-2xl font-bold mb-spade-md">
        Email verification
      </h2>
      <p v-if="status === 'pending'" class="text-spade-gray">
        Checking your verification link…
      </p>
      <p v-else-if="status === 'ok'" class="text-spade-green">
        {{ message }}
        <NuxtLink
          to="/login"
          class="block mt-spade-md text-spade-red font-semibold"
          >Continue to sign in</NuxtLink
        >
      </p>
      <p v-else class="text-spade-red">{{ message }}</p>
    </UCard>
  </div>
</template>
