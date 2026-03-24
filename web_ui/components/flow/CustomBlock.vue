<template>
    <div class="p-4 bg-white rounded-lg shadow-md border flex items-center justify-between space-x-1 relative w-64">
      <div class="flex">

        <span class="font-semibold">{{ name }}</span>
      </div>
      <div class="flex space-x-1">
        
          <UButton 
            @click="emit('edit', {...props})" 
            class="bg-blue-500 text-white p-1 rounded"
            icon="i-material-symbols:edit"
            ></UButton>
          <UButton 
            @click="emit('delete', {id: props.id})" 
            class="bg-red-500 text-white p-1 rounded"
            icon="i-material-symbols:delete-rounded"
            ></UButton>
        </div>
      
      <!-- VueFlow Handles -->
      <Handle type="target" :position="Position.Top" />
      <Handle type="source" :position="Position.Bottom" />
    </div>
  </template>
  
  <script setup lang="ts">
  import { defineProps, defineEmits } from 'vue';
  import { Handle, Position } from '@vue-flow/core';
  
  const props = defineProps<{ name: string, id: string }>();
  const emit = defineEmits(["edit", "delete"]);
  </script>
  