<script setup lang="ts">
const username = ref("");
const name = ref("");
const passwd = ref("");
const passwdConfirm = ref("");
const errorMessage = ref<string | null>(null);
const submitting = ref(false);

const auth = useAuth();

const handleSubmit = async () => {
  errorMessage.value = null;
  if (passwd.value !== passwdConfirm.value) {
    errorMessage.value = "Passwords do not match";
    return;
  }
  submitting.value = true;
  try {
    await auth.signUp(username.value, passwd.value, name.value);
    await navigateTo({ path: "/" });
  } catch (error: any) {
    console.error(error);
    errorMessage.value =
      error?.message || "Sign-up failed. Please try again.";
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
        <h2 class="font-heading text-2xl font-bold">Create your account</h2>
      </template>

      <form class="space-y-spade-md" @submit.prevent="handleSubmit">
        <UFormGroup label="Name" name="name">
          <UInput v-model="name" placeholder="Your name" autocomplete="name" />
        </UFormGroup>
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
            autocomplete="new-password"
          />
        </UFormGroup>
        <UFormGroup label="Confirm password" name="passwordConfirm" required>
          <UInput
            v-model="passwdConfirm"
            type="password"
            autocomplete="new-password"
          />
        </UFormGroup>
        <p v-if="errorMessage" class="text-sm text-spade-red">
          {{ errorMessage }}
        </p>
        <UButton type="submit" color="primary" block :loading="submitting">
          Create account
        </UButton>
      </form>

      <template #footer>
        <p class="text-sm text-spade-gray text-center">
          Already have an account?
          <NuxtLink
            to="/login"
            class="text-spade-red font-semibold hover:text-spade-red-light"
            >Sign in</NuxtLink
          >
        </p>
      </template>
    </UCard>
  </div>
</template>
