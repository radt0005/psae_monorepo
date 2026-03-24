<script setup lang="ts">

  const pb = usePB();



</script>

<template>
    <div>
      <AppHeader />
      <div v-if="pb.authStore.isValid">
        <Nav></Nav>
        <slot />
      </div>
      <div v-if="!pb.authStore.isValid">
        <Login></Login>
      </div>
      <AppFooter />
    </div>
  </template>