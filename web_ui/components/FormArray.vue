<template>
    <div>
      <label v-if="schema.title" class="block font-medium">{{ schema.title }}</label>
      <div v-for="(item, index) in modelValue" :key="index" class="flex items-center space-x-2">
        <SchemaField :schema="schema" 
                     :modelValue="item" 
                     @update:modelValue="(val) => updateItem(index, val)" />
                     <input class="input" type="text">
        <button type="button" @click="removeItem(index)" class="bg-red-500 text-white p-1 rounded">-</button>
      </div>
      <button type="button" @click="addItem" class="bg-blue-500 text-white p-2 rounded mt-2">Add Item</button>
    </div>
  </template>
  
  <script setup lang="ts">


  
  type Schema = {
    title?: string;
    items: {
      type: string;
    };
  };
  
  type ModelValueType = any;

  
  const props = defineProps<{ schema: Schema; modelValue: ModelValueType }>();
  const emit = defineEmits(["update:modelValue"]);
  
  const addItem = () => {
    let defaultValue: string | Object = "";
    if(props.schema.items) {
      defaultValue = props.schema.items.type === 'object' ? {} : '';
    }
   emit('update:modelValue', [...props.modelValue, defaultValue] as any);
  };
  
  const removeItem = (index: number) => {
    const updatedArray = [...props.modelValue];
    updatedArray.splice(index, 1);
    emit('update:modelValue', updatedArray as any);
  };
  
  const updateItem = (index: number, value: any) => {
    const updatedArray = [...props.modelValue];
    updatedArray[index] = value;
    emit('update:modelValue', updatedArray as any);
  };
  </script>
  