<script setup lang="ts">
const route = useRoute();
const config = useRuntimeConfig();

const userCode = ref((route.query.user_code as string) ?? "");
const status = ref<"idle" | "submitting" | "approved" | "denied" | "error">("idle");
const errorMessage = ref<string | null>(null);

const authBase = config.public.betterAuthUrl as string;

async function approve() {
  if (!userCode.value.trim()) return;
  status.value = "submitting";
  errorMessage.value = null;
  try {
    await $fetch(`${authBase}/api/auth/device/approve`, {
      method: "POST",
      body: { userCode: userCode.value.trim() },
      credentials: "include",
    });
    status.value = "approved";
  } catch (e: any) {
    errorMessage.value = e?.data?.message ?? e?.message ?? "Authorization failed.";
    status.value = "error";
  }
}

async function deny() {
  if (!userCode.value.trim()) return;
  status.value = "submitting";
  errorMessage.value = null;
  try {
    await $fetch(`${authBase}/api/auth/device/deny`, {
      method: "POST",
      body: { userCode: userCode.value.trim() },
      credentials: "include",
    });
    status.value = "denied";
  } catch (e: any) {
    errorMessage.value = e?.data?.message ?? e?.message ?? "Request failed.";
    status.value = "error";
  }
}
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
        <h2 class="font-heading text-2xl font-bold">Authorize device</h2>
        <p class="text-sm text-spade-gray">
          Enter the code displayed in your terminal to grant CLI access.
        </p>
      </template>

      <!-- Success -->
      <div v-if="status === 'approved'" class="text-center py-spade-md">
        <p class="text-spade-green font-semibold mb-spade-sm">Device authorized.</p>
        <p class="text-sm text-spade-gray">
          You can close this tab and return to your terminal.
        </p>
      </div>

      <!-- Denied -->
      <div v-else-if="status === 'denied'" class="text-center py-spade-md">
        <p class="text-spade-gray font-semibold">Access denied.</p>
        <p class="text-sm text-spade-gray mt-spade-xs">
          The request was rejected. Run <code>spade login</code> again if
          this was a mistake.
        </p>
      </div>

      <!-- Form -->
      <form
        v-else
        class="space-y-spade-md"
        @submit.prevent="approve"
      >
        <UFormGroup label="Device code" name="userCode" required>
          <UInput
            v-model="userCode"
            placeholder="XXXX-XXXX"
            autocomplete="off"
            autofocus
            class="font-mono tracking-widest text-center"
          />
        </UFormGroup>

        <p v-if="errorMessage" class="text-sm text-spade-red">
          {{ errorMessage }}
        </p>

        <div class="flex gap-spade-sm">
          <UButton
            type="submit"
            color="primary"
            block
            :loading="status === 'submitting'"
          >
            Authorize
          </UButton>
          <UButton
            type="button"
            color="gray"
            block
            :disabled="status === 'submitting'"
            @click="deny"
          >
            Deny
          </UButton>
        </div>
      </form>
    </UCard>
  </div>
</template>
