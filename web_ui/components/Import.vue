<script setup lang="ts">
    const emit = defineEmits(["close"])
    const data = ref("");

    const { yaml_to_nodes  } = useFlow();
    
    const handleConvert = () => {
        yaml_to_nodes(data.value);
        emit("close")
    }
    const handleClose = () => emit("close")
</script>

<template>
    <UCard>
        <template #header>
            <p>Import a Pipeline</p>
        </template>
        
        <UTextarea :rows="30" v-model="data" placeholder="Paste it here...."></UTextarea>

        <template #footer>
            <UButtonGroup> 
                <UButton @click="handleConvert">Parse it!</UButton>
                <UButton @click="handleClose">Cancel</UButton>
            </UButtonGroup>
        </template>
    </UCard>
    
</template>