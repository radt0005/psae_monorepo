<script setup lang="ts">
import { createAuthClient } from "better-auth/client";

const email = ref("");
const status = ref<"idle" | "sent" | "error">("idle");
const errorMessage = ref<string | null>(null);
const submitting = ref(false);

const handleSubmit = async () => {
  errorMessage.value = null;
  submitting.value = true;
  try {
    const config = useRuntimeConfig();
    const client = createAuthClient({
      baseURL: config.public.betterAuthUrl as string,
    });
    const res = await client.forgetPassword({
      email: email.value,
      redirectTo: "/reset-password",
    });
    if (res?.error) throw new Error(res.error.message);
    status.value = "sent";
  } catch (e: any) {
    errorMessage.value = e?.message || "Could not send reset email.";
    status.value = "error";
  } finally {
    submitting.value = false;
  }
};
</script>

<template>
  <div
    class="flex flex-col items-center justify-center min-h-screen px-spade-lg bg-spade-white"
  >
    <BrandSpadeLogo :width="40" :height="48" class="mb-spade-lg" />
    <UCard class="max-w-md w-full shadow-card">
      <template #header>
        <h2 class="font-heading text-2xl font-bold">Reset your password</h2>
      </template>
      <p v-if="status === 'sent'" class="text-spade-green">
        If an account exists for {{ email }}, you'll receive a reset link
        shortly.
      </p>
      <form
        v-else
        class="space-y-spade-md"
        @submit.prevent="handleSubmit"
      >
        <UFormGroup label="Email" name="email" required>
          <UInput v-model="email" type="email" placeholder="you@example.com" />
        </UFormGroup>
        <p v-if="errorMessage" class="text-sm text-spade-red">
          {{ errorMessage }}
        </p>
        <UButton type="submit" color="primary" block :loading="submitting">
          Send reset link
        </UButton>
      </form>
      <template #footer>
        <p class="text-sm text-spade-gray text-center">
          <NuxtLink
            to="/login"
            class="text-spade-red font-semibold hover:text-spade-red-light"
            >Back to sign in</NuxtLink
          >
        </p>
      </template>
    </UCard>
  </div>
</template>
