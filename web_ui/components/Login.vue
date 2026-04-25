<script setup lang="ts">
const username = ref("");
const passwd = ref("");
const errorMessage = ref<string | null>(null);
const submitting = ref(false);

const auth = useAuth();

const handleSubmit = async () => {
  errorMessage.value = null;
  submitting.value = true;
  try {
    await auth.signIn(username.value, passwd.value);
    await navigateTo({ path: "/" });
  } catch (error: any) {
    console.error(error);
    errorMessage.value =
      error?.message || "Could not sign in. Please try again.";
  } finally {
    submitting.value = false;
  }
};
</script>

<template>
  <div
    class="flex flex-col items-center justify-center min-h-screen px-spade-lg bg-spade-white"
  >
    <NuxtLink
      to="/"
      class="flex items-center gap-spade-sm font-heading text-3xl font-bold text-spade-black mb-spade-lg hover:no-underline"
    >
      <BrandSpadeLogo :width="40" :height="48" />
      Spade
    </NuxtLink>
    <UCard class="max-w-md w-full shadow-card">
      <template #header>
        <h2 class="font-heading text-2xl font-bold">Sign in</h2>
        <p class="text-sm text-spade-gray">
          Welcome back. Enter your credentials to continue.
        </p>
      </template>

      <form class="space-y-spade-md" @submit.prevent="handleSubmit">
        <UFormGroup label="Email" name="email" required>
          <UInput
            v-model="username"
            type="email"
            placeholder="you@example.com"
            autocomplete="email"
          />
        </UFormGroup>
        <UFormGroup label="Password" name="password" required>
          <UInput
            v-model="passwd"
            type="password"
            placeholder="••••••••"
            autocomplete="current-password"
          />
        </UFormGroup>
        <p v-if="errorMessage" class="text-sm text-spade-red">
          {{ errorMessage }}
        </p>
        <UButton type="submit" color="primary" block :loading="submitting">
          Sign in
        </UButton>
      </form>

      <template #footer>
        <p class="text-sm text-spade-gray text-center">
          No account?
          <NuxtLink
            to="/signup"
            class="text-spade-red font-semibold hover:text-spade-red-light"
            >Create one</NuxtLink
          >
          ·
          <NuxtLink
            to="/forgot-password"
            class="text-spade-red font-semibold hover:text-spade-red-light"
            >Forgot password?</NuxtLink
          >
        </p>
      </template>
    </UCard>
  </div>
</template>
