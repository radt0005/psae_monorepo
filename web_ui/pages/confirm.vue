<script setup lang="ts">
    const pb = usePB();

    onMounted(() => {
      if(pb.value.authStore.isValid) {
        navigateTo("/");
      }
    })
</script>

<template>
  <div>Waiting for login...</div>
</template>