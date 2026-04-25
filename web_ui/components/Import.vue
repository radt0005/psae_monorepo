<script setup lang="ts">
    const emit = defineEmits(["close"])
    const data = ref("");

    const flow = useFlow();
    const errorMessage = ref<string | null>(null);

    const handleConvert = () => {
        errorMessage.value = null;
        try {
            flow.yamlToNodes(data.value);
            emit("close")
        } catch (e: any) {
            errorMessage.value = e?.message ?? "Failed to parse pipeline YAML";
        }
    }
    const handleClose = () => emit("close")
</script>

<template>
    <UCard>
        <template #header>
            <p>Import a Pipeline</p>
        </template>
        
        <UTextarea :rows="30" v-model="data" placeholder="Paste it here...."></UTextarea>
        <p v-if="errorMessage" class="text-red-600 mt-2">{{ errorMessage }}</p>

        <template #footer>
            <UButtonGroup> 
                <UButton @click="handleConvert">Parse it!</UButton>
                <UButton @click="handleClose">Cancel</UButton>
            </UButtonGroup>
        </template>
    </UCard>
    
</template>