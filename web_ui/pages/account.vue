<script setup lang="ts">
import { createAuthClient } from "better-auth/client";

const auth = useAuth();

const currentPassword = ref("");
const newPassword = ref("");
const newPasswordConfirm = ref("");
const message = ref<string | null>(null);
const errorMessage = ref<string | null>(null);
const submitting = ref(false);

const handleChangePassword = async () => {
  errorMessage.value = null;
  message.value = null;
  if (newPassword.value !== newPasswordConfirm.value) {
    errorMessage.value = "Passwords do not match.";
    return;
  }
  if (newPassword.value.length < 8) {
    errorMessage.value = "New password must be at least 8 characters.";
    return;
  }
  submitting.value = true;
  try {
    const config = useRuntimeConfig();
    const client = createAuthClient({
      baseURL: config.public.betterAuthUrl as string,
    });
    const res = await client.changePassword({
      currentPassword: currentPassword.value,
      newPassword: newPassword.value,
    });
    if (res?.error) throw new Error(res.error.message);
    message.value = "Password updated.";
    currentPassword.value = "";
    newPassword.value = "";
    newPasswordConfirm.value = "";
  } catch (e: any) {
    errorMessage.value = e?.message || "Could not update password.";
  } finally {
    submitting.value = false;
  }
};

const handleSignOut = async () => {
  await auth.signOut();
  await navigateTo("/login");
};
</script>

<template>
  <div class="max-w-2xl mx-auto px-spade-lg py-spade-xl">
    <h1 class="font-heading text-3xl font-bold mb-spade-lg">Account</h1>

    <UCard class="mb-spade-lg">
      <template #header>
        <h2 class="font-heading text-xl font-bold">Profile</h2>
      </template>
      <dl class="grid grid-cols-[140px_1fr] gap-spade-sm text-sm">
        <dt class="text-spade-gray">Name</dt>
        <dd>{{ auth.user.value?.name || "—" }}</dd>
        <dt class="text-spade-gray">Email</dt>
        <dd>{{ auth.user.value?.email }}</dd>
        <dt class="text-spade-gray">Role</dt>
        <dd>{{ auth.user.value?.role || "user" }}</dd>
      </dl>
    </UCard>

    <UCard class="mb-spade-lg">
      <template #header>
        <h2 class="font-heading text-xl font-bold">Change password</h2>
      </template>
      <form class="space-y-spade-md" @submit.prevent="handleChangePassword">
        <UFormGroup label="Current password" required>
          <UInput
            v-model="currentPassword"
            type="password"
            autocomplete="current-password"
          />
        </UFormGroup>
        <UFormGroup label="New password" required>
          <UInput
            v-model="newPassword"
            type="password"
            autocomplete="new-password"
          />
        </UFormGroup>
        <UFormGroup label="Confirm new password" required>
          <UInput
            v-model="newPasswordConfirm"
            type="password"
            autocomplete="new-password"
          />
        </UFormGroup>
        <p v-if="message" class="text-spade-green text-sm">{{ message }}</p>
        <p v-if="errorMessage" class="text-spade-red text-sm">
          {{ errorMessage }}
        </p>
        <UButton type="submit" color="primary" :loading="submitting">
          Update password
        </UButton>
      </form>
    </UCard>

    <UCard>
      <template #header>
        <h2 class="font-heading text-xl font-bold">Session</h2>
      </template>
      <UButton
        color="red"
        variant="outline"
        icon="i-heroicons-arrow-right-on-rectangle"
        @click="handleSignOut"
      >
        Sign out
      </UButton>
    </UCard>
  </div>
</template>
