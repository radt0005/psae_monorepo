<template>
  <form @submit.prevent="submitForm" class="p-4 space-y-4">
    <SchemaField
      v-for="(value, key) in schema.properties"
      :key="key"
      :name="key.toString()"
      :schema="value"
      :modelValue="formData[key]"
      @update:modelValue="(val) => updateField(key, val)"
    />
  </form>
</template>

<script setup lang="ts">
import { ref, watch, defineProps } from "vue";
import SchemaField from "./SchemaField.vue";


const props = defineProps<{ schema: Record<string, any>, targetId: string | null}>();
const { getArgsById } = useFlow();
const formData = ref<Record<string, any>>(initializeFormData(props.schema));
const emit = defineEmits(["update:modelValue"])




function initializeFormData(schema: Record<string, any>) {
  

  
  if(props.targetId === null) {
    
    const data: Record<string, any> = {};
    for (const key in schema.properties) {
      data[key] = getDefaultValue(schema.properties[key]);
    }
    //A.argsById[id] = data
    console.log(`No data found, initialized form to default values: ${data}`)
    return data;
  } else {
    const savedArgsOrNull = getArgsById(props.targetId)
    console.log(`Stored data found, initializing form to: ${savedArgsOrNull}`)
    return savedArgsOrNull
  }
}

function getDefaultValue(fieldSchema: any) {
  if (fieldSchema.type === "array") return [];
  if (fieldSchema.type === "object") return {};
  return "";
}

function updateField(key: string | number, value: any) {
  formData.value[key] = value;
  emit("update:modelValue", formData.value);
}

function submitForm() {
  // save the Args in the store here
  console.log("Form submitted:", formData.value);
  emit("update:modelValue", formData.value  )
}


// Reinitialize form data if schema changes
watch(() => props.schema, (newSchema) => {
  formData.value = initializeFormData(newSchema);
}, { deep: true });
</script>
