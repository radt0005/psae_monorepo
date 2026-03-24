<script setup lang="ts">
    const pb = usePB();

    const props = defineProps<{id: string}>();

    try {

        const response = await pb.value.collection("blocks").getOne(props.id);

        console.log(response);
        
    } catch(error) {
        console.error(error);
    }


</script>

<template>
    <div>
        <label>
            <p>Output File Name (Optional)</p>
            <UInput type="text"></UInput>
        </label>
    </div>
</template>