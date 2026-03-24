<template>
    <div>
      <p>{{ name }}</p>
        <label v-if="schema.title" class="block font-medium">{{ schema.title }}</label>
      
      <FormInput v-if="isPrimitive" 
                 :schema="schema" 
                 :modelValue="modelValue" 
                 @update:modelValue="$emit('update:modelValue', $event)" />
      
      <FormArray v-else-if="schema.type === 'array'" 
                 :schema="schema.items" 
                 :modelValue="modelValue" 
                 @update:modelValue="$emit('update:modelValue', $event)" />
      
      <div v-else-if="schema.type === 'object'" class="p-4 border rounded">
        <SchemaField v-for="(subSchema, key) in schema.properties" 
                     :key="key" 
                     :schema="subSchema" 
                     :modelValue="modelValue[key]" 
                     @update:modelValue="(val) => updateObjectField(key, val)" />
      </div>
    </div>
  </template>
  
  <script>
  import FormInput from './FormInput.vue';
  import FormArray from './FormArray.vue';
  
  export default {
    components: { FormInput, FormArray },
    props: {
      schema: Object,
      modelValue: [String, Number, Boolean, Array, Object],
      name: String
    },
    computed: {
      isPrimitive() {
        return ['string', 'number', 'boolean'].includes(this.schema.type);
      }
    },
    methods: {
      updateObjectField(key, value) {
        const updatedValue = { ...this.modelValue, [key]: value };
        this.$emit('update:modelValue', updatedValue);
      }
    }
  };
  </script>
  