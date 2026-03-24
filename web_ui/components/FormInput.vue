<template>
    <input v-if="schema.type === 'number'" 
           :type="schema.attrs?.type || (schema.type === 'number' ? 'number' : 'text')" 
           class="border p-2 rounded w-full" 
           :placeholder="schema.attrs?.placeholder" 
           :value="modelValue" 
           @input="$emit('update:modelValue', $event.target.value)" />
    
    <textarea v-if="schema.type === 'string'"
    class="border p-2 rounded w-full" 
           :placeholder="schema.attrs?.placeholder" 
           :value="modelValue" 
           @input="$emit('update:modelValue', $event.target.value)"  ></textarea>
    <select v-else-if="schema.enum" 
            class="border p-2 rounded w-full" 
            :value="modelValue" 
            @change="$emit('update:modelValue', $event.target.value)">
      <option v-for="option in schema.enum" :key="option" :value="option">{{ option }}</option>
    </select>
    <input type="checkbox" v-if="schema.type === 'boolean'" 
      class="p-2"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value === 'false')"
      :checked="modelValue"
      />
  </template>
  
  <script>
  export default {
    props: {
      schema: Object,
      modelValue: [String, Number, Boolean]
    }
  };
  </script>
  